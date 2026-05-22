package auth

import (
	"errors"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable   = errors.New("auth store unavailable")
	ErrEmailAlreadyExists = errors.New("auth email already exists")
	ErrUserNotFound       = errors.New("auth user not found")
	ErrInvalidOTP         = errors.New("invalid or expired OTP")
	ErrOTPAlreadyUsed     = errors.New("OTP has already been used")
	ErrOTPExpired         = errors.New("OTP has expired")
)

type AuthService struct {
	DB *gorm.DB
}

func (s *AuthService) CreateUser(user Auth) (*Auth, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	if user.Role == "" {
		user.Role = "User"
	}

	if err := s.DB.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}

	return &user, nil
}

func (s *AuthService) GetUser(email string) (*Auth, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	var user Auth
	if err := s.DB.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) GetUserByID(id int) (*Auth, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	var user Auth
	if err := s.DB.Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// CreatePasswordResetOTP creates a new OTP for password reset
func (s *AuthService) CreatePasswordResetOTP(email string) (*PasswordResetOTP, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	// Verify user exists
	user, err := s.GetUser(email)
	if err != nil {
		return nil, err
	}

	// Generate OTP
	otp, err := util.GenerateOTP()
	if err != nil {
		return nil, err
	}

	// Create OTP record with 10-minute expiry
	passwordReset := &PasswordResetOTP{
		UserID:    user.ID,
		Email:     email,
		OTP:       otp,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		IsUsed:    false,
	}

	if err := s.DB.Create(passwordReset).Error; err != nil {
		return nil, err
	}

	return passwordReset, nil
}

// VerifyPasswordResetOTP verifies the OTP and resets the password
func (s *AuthService) VerifyPasswordResetOTP(email, otp, newPassword string) (*Auth, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	// Find the latest OTP for the email
	var passwordReset PasswordResetOTP
	if err := s.DB.Where("email = ? AND otp = ?", email, otp).
		Order("created_at DESC").
		First(&passwordReset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidOTP
		}
		return nil, err
	}

	// Check if OTP has been used
	if passwordReset.IsUsed {
		return nil, ErrOTPAlreadyUsed
	}

	// Check if OTP has expired
	if time.Now().After(passwordReset.ExpiresAt) {
		return nil, ErrOTPExpired
	}

	// Get the user
	user, err := s.GetUser(email)
	if err != nil {
		return nil, err
	}

	// Hash the new password
	hashedPassword, err := util.HashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	// Update user password in a transaction
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		// Update user password
		if err := tx.Model(user).Update("password", hashedPassword).Error; err != nil {
			return err
		}

		// Mark OTP as used
		if err := tx.Model(&passwordReset).Update("is_used", true).Error; err != nil {
			return err
		}

		// Clean up old OTPs for this user (optional but recommended)
		if err := tx.Where("user_id = ? AND expires_at < ?", user.ID, time.Now()).
			Delete(&PasswordResetOTP{}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	user.Password = hashedPassword

	return user, nil
}

// GetUnusedOTPByEmail retrieves an unused OTP for the given email
func (s *AuthService) GetUnusedOTPByEmail(email string) (*PasswordResetOTP, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var passwordReset PasswordResetOTP
	if err := s.DB.Where("email = ? AND is_used = ? AND expires_at > ?", email, false, time.Now()).
		Order("created_at DESC").
		First(&passwordReset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidOTP
		}
		return nil, err
	}

	return &passwordReset, nil
}

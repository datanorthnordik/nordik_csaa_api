package auth

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable   = errors.New("auth store unavailable")
	ErrEmailAlreadyExists = errors.New("auth email already exists")
	ErrUserNotFound       = errors.New("auth user not found")
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

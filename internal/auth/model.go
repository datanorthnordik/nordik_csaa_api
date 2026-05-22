package auth

import "time"

type Auth struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	FirstName string    `gorm:"size:100;not null;column:firstname" json:"firstname"`
	LastName  string    `gorm:"size:100;not null;column:lastname" json:"lastname"`
	Email     string    `gorm:"size:100;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	FirstName    string `json:"firstname"`
	LastName     string `json:"lastname"`
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
}

type PasswordResetOTP struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"not null;index" json:"user_id"`
	Email     string    `gorm:"not null;index" json:"email"`
	OTP       string    `gorm:"not null" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	IsUsed    bool      `gorm:"default:false" json:"is_used"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Email    string `json:"email" binding:"required,email"`
	OTP      string `json:"otp" binding:"required,len=6"`
	Password string `json:"password" binding:"required,min=6"`
}

func (Auth) TableName() string {
	return "users"
}

func (PasswordResetOTP) TableName() string {
	return "password_reset_otps"
}

package auth

type AuthServicePort interface {
	CreateUser(user Auth) (*Auth, error)
	GetUser(email string) (*Auth, error)
	GetUserByID(id int) (*Auth, error)
	CreatePasswordResetOTP(email string) (*PasswordResetOTP, error)
	VerifyPasswordResetOTP(email, otp, newPassword string) (*Auth, error)
	GetUnusedOTPByEmail(email string) (*PasswordResetOTP, error)
}

var _ AuthServicePort = (*AuthService)(nil)

package auth

type AuthServicePort interface {
	CreateUser(user Auth) (*Auth, error)
	GetUser(email string) (*Auth, error)
	GetUserByID(id int) (*Auth, error)
}

var _ AuthServicePort = (*AuthService)(nil)

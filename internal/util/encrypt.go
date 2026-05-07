package util

import "golang.org/x/crypto/bcrypt"

var bcryptGenerateFromPassword = bcrypt.GenerateFromPassword

func HashPassword(password string) (string, error) {
	hashed, err := bcryptGenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func VerifyPassword(password, hashed string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
}

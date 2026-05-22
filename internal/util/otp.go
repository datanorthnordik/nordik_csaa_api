package util

import (
	"crypto/rand"
	"fmt"
)

// GenerateOTP generates a 6-digit OTP
func GenerateOTP() (string, error) {
	// Generate 3 random bytes which gives us 0-16777215 (24 bits)
	// This is enough to generate a 6-digit number
	b := make([]byte, 3)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// Convert bytes to integer and take modulo 1000000 to get 0-999999
	num := (int(b[0]) << 16) | (int(b[1]) << 8) | int(b[2])
	otp := num % 1000000

	// Pad with zeros to always get 6 digits
	return fmt.Sprintf("%06d", otp), nil
}

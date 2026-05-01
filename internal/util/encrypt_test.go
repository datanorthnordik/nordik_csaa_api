package util

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "secret123" {
		t.Fatal("expected hash to differ from plaintext password")
	}
	if err := VerifyPassword("secret123", hash); err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if err := VerifyPassword("wrong", hash); err == nil {
		t.Fatal("expected wrong password to fail verification")
	}
}

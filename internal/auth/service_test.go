package auth

import (
	"errors"
	"testing"
)

func TestAuthServiceReturnsStoreUnavailableWithoutDB(t *testing.T) {
	service := &AuthService{}

	if _, err := service.CreateUser(Auth{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected CreateUser to return ErrStoreUnavailable, got %v", err)
	}

	if _, err := service.GetUser("ada@example.com"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetUser to return ErrStoreUnavailable, got %v", err)
	}

	if _, err := service.GetUserByID(42); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetUserByID to return ErrStoreUnavailable, got %v", err)
	}
}

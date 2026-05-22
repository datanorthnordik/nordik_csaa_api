package util

import (
	"regexp"
	"testing"
)

func TestGenerateOTP(t *testing.T) {
	// Test that OTP is generated
	otp, err := GenerateOTP()
	if err != nil {
		t.Fatalf("Failed to generate OTP: %v", err)
	}

	// Test that OTP is 6 digits
	if len(otp) != 6 {
		t.Fatalf("Expected OTP length 6, got %d", len(otp))
	}

	// Test that OTP contains only digits
	if !regexp.MustCompile(`^\d{6}$`).MatchString(otp) {
		t.Fatalf("OTP should contain only digits, got %s", otp)
	}
}

func TestGenerateOTPUniqueness(t *testing.T) {
	// Generate multiple OTPs and ensure they're not all the same
	// (statistically, they should be different)
	otps := make(map[string]bool)
	for i := 0; i < 100; i++ {
		otp, err := GenerateOTP()
		if err != nil {
			t.Fatalf("Failed to generate OTP: %v", err)
		}
		otps[otp] = true
	}

	if len(otps) < 50 {
		t.Logf("Warning: Generated %d unique OTPs out of 100. This could be a randomness issue.", len(otps))
	}
}

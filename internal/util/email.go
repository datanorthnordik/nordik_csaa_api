package util

import (
	"fmt"
	"log"
	"net/smtp"
	"nordikcsaaapi/internal/config"
)

// EmailService handles sending emails
type EmailService struct {
	Config *config.Config
}

// NewEmailService creates a new email service instance
func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		Config: cfg,
	}
}

// SendPasswordResetEmail sends a password reset OTP email asynchronously
func (es *EmailService) SendPasswordResetEmail(email, otp, firstName string) error {
	// Run in a goroutine to avoid blocking the main request
	go func() {
		if err := es.sendPasswordResetEmailSync(email, otp, firstName); err != nil {
			log.Printf("Failed to send password reset email to %s: %v", email, err)
		}
	}()
	return nil
}

// sendPasswordResetEmailSync sends the email synchronously (internal use)
func (es *EmailService) sendPasswordResetEmailSync(email, otp, firstName string) error {
	// Gmail SMTP configuration
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	// Create the email body
	subject := "Password Reset OTP"
	body := fmt.Sprintf(`
Dear %s,

You have requested to reset your password. Please use the following One-Time Password (OTP) to reset your password:

OTP: %s

This OTP is valid for 10 minutes only. If you did not request this, please ignore this email and your password will remain unchanged.

Please note:
- Do not share this OTP with anyone
- This OTP is confidential and for your security
- If you continue to experience issues, please contact our support team

Best regards,
Nordik Team
`, firstName, otp)

	// Prepare the email message
	msg := []byte("To: " + email + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body)

	// Set up authentication
	auth := smtp.PlainAuth("", es.Config.GmailUser, es.Config.GmailPass, smtpHost)

	// Send the email
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, es.Config.GmailUser, []string{email}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

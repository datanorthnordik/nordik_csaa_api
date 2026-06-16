package util

import (
	"fmt"
	"log"
	"net/smtp"
	"nordikcsaaapi/internal/config"
	"strings"
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

// SendEmail sends a plain text email synchronously to one or more recipients.
func (es *EmailService) SendEmail(to []string, subject string, body string) error {
	return es.sendPlainTextEmailSync(to, subject, body)
}

// sendPasswordResetEmailSync sends the email synchronously (internal use)
func (es *EmailService) sendPasswordResetEmailSync(email, otp, firstName string) error {
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

	return es.sendPlainTextEmailSync([]string{email}, subject, body)
}

func (es *EmailService) sendPlainTextEmailSync(to []string, subject string, body string) error {
	recipients := sanitizeEmailRecipients(to)
	if len(recipients) == 0 {
		return fmt.Errorf("at least one email recipient is required")
	}
	if es == nil || es.Config == nil {
		return fmt.Errorf("email configuration is not available")
	}
	if strings.TrimSpace(es.Config.GmailUser) == "" || strings.TrimSpace(es.Config.GmailPass) == "" {
		return fmt.Errorf("gmail smtp credentials are not configured")
	}

	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	msg := []byte("To: " + strings.Join(recipients, ", ") + "\r\n" +
		"Subject: " + strings.TrimSpace(subject) + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body)

	auth := smtp.PlainAuth("", es.Config.GmailUser, es.Config.GmailPass, smtpHost)
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, es.Config.GmailUser, recipients, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func sanitizeEmailRecipients(to []string) []string {
	seen := make(map[string]struct{}, len(to))
	recipients := make([]string, 0, len(to))
	for _, item := range to {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		recipients = append(recipients, item)
	}
	return recipients
}

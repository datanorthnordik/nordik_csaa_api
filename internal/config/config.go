package config

import (
	"os"
)

type Config struct {
	DatabaseURL string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	JWTSecret   string
	SMTPKey     string
	GmailUser   string
	GmailPass   string
}

func LoadConfig() Config {
	return Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		DBHost:      os.Getenv("DB_HOST"),
		DBPort:      os.Getenv("DB_PORT"),
		DBUser:      os.Getenv("DB_USER"),
		DBPassword:  os.Getenv("DB_PASSWORD"),
		DBName:      os.Getenv("DB_NAME"),
		DBSSLMode:   os.Getenv("DB_SSLMODE"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		SMTPKey:     os.Getenv("SMTP_KEY"),
		GmailUser:   os.Getenv("GMAIL_USER"),
		GmailPass:   os.Getenv("GMAIL_APP_PASSWORD"),
	}
}

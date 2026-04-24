package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName           string
	Environment       string
	Port              string
	BaseURL           string
	LogLevel          slog.Level
	RequestTimeout    time.Duration
	CORSAllowedOrigin []string
}

func Load() Config {
	return Config{
		AppName:           env("APP_NAME", "nordikcsaaapi"),
		Environment:       env("APP_ENV", "local"),
		Port:              env("APP_PORT", "8080"),
		BaseURL:           env("APP_BASE_URL", "http://localhost:8080"),
		LogLevel:          parseLogLevel(env("LOG_LEVEL", "info")),
		RequestTimeout:    time.Duration(envInt("REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
		CORSAllowedOrigin: splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")),
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

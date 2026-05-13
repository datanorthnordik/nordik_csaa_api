package auth

import (
	"errors"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RequireAPIKey(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAPIKey(c, configuredAPIKey(cfg)) {
			return
		}
		c.Next()
	}
}

func RequireBearerAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireBearerAuth(c, configuredJWTSecret(cfg)) {
			return
		}
		c.Next()
	}
}

func configuredAPIKey(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.APIKey)
}

func configuredJWTSecret(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.JWTSecret)
}

func requireAPIKey(c *gin.Context, expectedKey string) bool {
	if expectedKey == "" {
		apiresponse.WriteServiceUnavailable(c, "API key authentication is temporarily unavailable")
		return false
	}

	providedKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if providedKey == "" {
		apiresponse.WriteUnauthorized(c, "missing_api_key", "Missing API key")
		return false
	}
	if providedKey != expectedKey {
		apiresponse.WriteUnauthorized(c, "invalid_api_key", "Invalid API key")
		return false
	}

	return true
}

func requireBearerAuth(c *gin.Context, secret string) bool {
	tokenString, err := bearerToken(c.GetHeader("Authorization"))
	if err != nil {
		apiresponse.WriteUnauthorized(c, "missing_bearer_token", err.Error())
		return false
	}
	if secret == "" {
		apiresponse.WriteServiceUnavailable(c, "Authentication service is temporarily unavailable")
		return false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		apiresponse.WriteUnauthorized(c, "invalid_access_token", "Invalid access token")
		return false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		apiresponse.WriteUnauthorized(c, "invalid_access_token", "Invalid access token")
		return false
	}

	if userID, ok := claimInt(claims["user_id"]); ok {
		c.Set("auth_user_id", userID)
	}
	if email, ok := claims["email"].(string); ok {
		c.Set("auth_email", email)
	}
	if role, ok := claims["role"].(string); ok {
		c.Set("auth_role", role)
	}

	return true
}

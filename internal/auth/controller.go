package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"nordikcsaaapi/internal/config"
	"nordikcsaaapi/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthController struct {
	AuthService AuthServicePort
	CFG         *config.Config
}

type signUpRequest struct {
	FirstName string `json:"firstname" binding:"required"`
	LastName  string `json:"lastname" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
}

type loginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"rememberMe"`
}

func (ac *AuthController) SignUp(c *gin.Context) {
	var req signUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	password, err := util.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	user, err := ac.AuthService.CreateUser(Auth{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  password,
	})
	if err != nil {
		if errors.Is(err, ErrStoreUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service is temporarily unavailable"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user": gin.H{
			"id":        user.ID,
			"firstname": user.FirstName,
			"lastname":  user.LastName,
			"email":     user.Email,
			"role":      user.Role,
		},
	})
}

func (ac *AuthController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := ac.AuthService.GetUser(req.Email)
	if err != nil {
		if errors.Is(err, ErrStoreUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service is temporarily unavailable"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if err := util.VerifyPassword(req.Password, user.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	accessToken, err := ac.signToken(user, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	refreshDuration := 24 * time.Hour
	if req.RememberMe {
		refreshDuration = 30 * 24 * time.Hour
	}
	refreshToken, err := ac.signToken(user, refreshDuration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"data": LoginResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ID:           user.ID,
			FirstName:    user.FirstName,
			LastName:     user.LastName,
			Email:        user.Email,
			Role:         user.Role,
		},
	})
}

func (ac *AuthController) Refresh(c *gin.Context) {
	refreshToken, err := bearerToken(c.GetHeader("Authorization"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(ac.CFG.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	userID, ok := claimInt(claims["user_id"])
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	user, err := ac.AuthService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, ErrStoreUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service is temporarily unavailable"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	accessToken, err := ac.signToken(user, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Access token refreshed",
		"accessToken": accessToken,
	})
}

func (ac *AuthController) signToken(user *Auth, duration time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"exp":     time.Now().Add(duration).Unix(),
	})
	return token.SignedString([]byte(ac.CFG.JWTSecret))
}

func bearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", errors.New("Missing bearer token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", errors.New("Missing bearer token")
	}
	return token, nil
}

func claimInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

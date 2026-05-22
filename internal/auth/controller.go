package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/config"
	"nordikcsaaapi/internal/httpapi"
	"nordikcsaaapi/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthController struct {
	AuthService AuthServicePort
	CFG         *config.Config
}

type passwordResetEmailSender interface {
	SendPasswordResetEmail(email, otp, firstName string) error
}

var newPasswordResetEmailSender = func(cfg *config.Config) passwordResetEmailSender {
	return util.NewEmailService(cfg)
}

const passwordResetSentMessage = "If an account with this email exists, a password reset OTP has been sent"

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
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	password, err := util.HashPassword(req.Password)
	if err != nil {
		httpapi.LogRequestError(c, "auth", err)
		apiresponse.WriteInternalError(c)
		return
	}

	user, err := ac.AuthService.CreateUser(Auth{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  password,
	})
	if err != nil {
		httpapi.HandleError(c, "auth", err,
			httpapi.ServiceUnavailableRule("Authentication service is temporarily unavailable", ErrStoreUnavailable),
			httpapi.ErrorRule{
				Match: func(err error) bool {
					return errors.Is(err, ErrEmailAlreadyExists)
				},
				Handle: func(c *gin.Context, _ error) {
					apiresponse.WriteConflict(c, "An account with this email already exists")
				},
			},
		)
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
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	user, err := ac.AuthService.GetUser(req.Email)
	if err != nil {
		if errors.Is(err, ErrStoreUnavailable) {
			httpapi.HandleError(c, "auth", err,
				httpapi.ServiceUnavailableRule("Authentication service is temporarily unavailable", ErrStoreUnavailable),
			)
			return
		}
		httpapi.LogRequestError(c, "auth", err)
		apiresponse.WriteUnauthorized(c, "invalid_credentials", "Invalid email or password")
		return
	}

	if err := util.VerifyPassword(req.Password, user.Password); err != nil {
		apiresponse.WriteUnauthorized(c, "invalid_credentials", "Invalid email or password")
		return
	}

	accessToken, err := ac.signToken(user, 15*time.Minute)
	if err != nil {
		httpapi.LogRequestError(c, "auth", err)
		apiresponse.WriteInternalError(c)
		return
	}

	refreshDuration := 24 * time.Hour
	if req.RememberMe {
		refreshDuration = 30 * 24 * time.Hour
	}
	refreshToken, err := ac.signToken(user, refreshDuration)
	if err != nil {
		httpapi.LogRequestError(c, "auth", err)
		apiresponse.WriteInternalError(c)
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
		apiresponse.WriteUnauthorized(c, "missing_bearer_token", err.Error())
		return
	}

	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(ac.CFG.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		apiresponse.WriteUnauthorized(c, "invalid_refresh_token", "Invalid refresh token")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		apiresponse.WriteUnauthorized(c, "invalid_refresh_token", "Invalid refresh token")
		return
	}

	userID, ok := claimInt(claims["user_id"])
	if !ok {
		apiresponse.WriteUnauthorized(c, "invalid_refresh_token", "Invalid refresh token")
		return
	}

	user, err := ac.AuthService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, ErrStoreUnavailable) {
			httpapi.HandleError(c, "auth", err,
				httpapi.ServiceUnavailableRule("Authentication service is temporarily unavailable", ErrStoreUnavailable),
			)
			return
		}
		httpapi.LogRequestError(c, "auth", err)
		apiresponse.WriteUnauthorized(c, "invalid_refresh_token", "Invalid refresh token")
		return
	}

	accessToken, err := ac.signToken(user, 15*time.Minute)
	if err != nil {
		httpapi.LogRequestError(c, "auth", err)
		apiresponse.WriteInternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Access token refreshed",
		"accessToken": accessToken,
	})
}

func (ac *AuthController) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	// Create OTP
	passwordReset, err := ac.AuthService.CreatePasswordResetOTP(req.Email)
	if err != nil {
		if errors.Is(err, ErrStoreUnavailable) {
			httpapi.HandleError(c, "auth", err,
				httpapi.ServiceUnavailableRule("Authentication service is temporarily unavailable", ErrStoreUnavailable),
			)
			return
		}
		if errors.Is(err, ErrUserNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"message": passwordResetSentMessage,
			})
			return
		}
		httpapi.LogRequestError(c, "auth", err)
		apiresponse.WriteInternalError(c)
		return
	}

	firstName := ""
	user, err := ac.AuthService.GetUser(req.Email)
	if err != nil {
		httpapi.LogRequestError(c, "auth", err)
	} else {
		firstName = user.FirstName
	}

	emailService := newPasswordResetEmailSender(ac.CFG)
	_ = emailService.SendPasswordResetEmail(req.Email, passwordReset.OTP, firstName)

	c.JSON(http.StatusOK, gin.H{
		"message": passwordResetSentMessage,
	})
}

func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	// Verify OTP and reset password
	user, err := ac.AuthService.VerifyPasswordResetOTP(req.Email, req.OTP, req.Password)
	if err != nil {
		httpapi.HandleError(c, "auth", err,
			httpapi.ServiceUnavailableRule("Authentication service is temporarily unavailable", ErrStoreUnavailable),
			httpapi.ErrorRule{
				Match: isPasswordResetValidationError,
				Handle: func(c *gin.Context, err error) {
					apiresponse.WriteValidationError(c, passwordResetValidationMessage(err))
				},
			},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset successfully",
		"user": gin.H{
			"id":        user.ID,
			"firstname": user.FirstName,
			"lastname":  user.LastName,
			"email":     user.Email,
			"role":      user.Role,
		},
	})
}

func isPasswordResetValidationError(err error) bool {
	return errors.Is(err, ErrInvalidOTP) ||
		errors.Is(err, ErrOTPAlreadyUsed) ||
		errors.Is(err, ErrOTPExpired) ||
		errors.Is(err, ErrUserNotFound)
}

func passwordResetValidationMessage(err error) string {
	switch {
	case errors.Is(err, ErrOTPAlreadyUsed):
		return "OTP has already been used. Please request a new OTP"
	case errors.Is(err, ErrOTPExpired):
		return "OTP has expired. Please request a new OTP"
	default:
		return "Invalid OTP"
	}
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

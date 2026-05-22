package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/config"
	"nordikcsaaapi/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type fakeAuthService struct {
	usersByEmail              map[string]*Auth
	usersByID                 map[int]*Auth
	createErr                 error
	getUserErr                error
	getUserByIDErr            error
	createPasswordResetOTPErr error
	verifyPasswordResetOTPErr error
	otpsByEmail               map[string]*PasswordResetOTP
}

func newFakeAuthService() *fakeAuthService {
	return &fakeAuthService{
		usersByEmail: map[string]*Auth{},
		usersByID:    map[int]*Auth{},
		otpsByEmail:  map[string]*PasswordResetOTP{},
	}
}

func (s *fakeAuthService) CreateUser(user Auth) (*Auth, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	user.ID = len(s.usersByID) + 1
	if user.Role == "" {
		user.Role = "User"
	}
	copy := user
	s.usersByEmail[user.Email] = &copy
	s.usersByID[user.ID] = &copy
	return &copy, nil
}

func (s *fakeAuthService) GetUser(email string) (*Auth, error) {
	if s.getUserErr != nil {
		return nil, s.getUserErr
	}
	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return user, nil
}

func (s *fakeAuthService) GetUserByID(id int) (*Auth, error) {
	if s.getUserByIDErr != nil {
		return nil, s.getUserByIDErr
	}
	user, ok := s.usersByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return user, nil
}

func (s *fakeAuthService) CreatePasswordResetOTP(email string) (*PasswordResetOTP, error) {
	if s.createPasswordResetOTPErr != nil {
		return nil, s.createPasswordResetOTPErr
	}
	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	otp := &PasswordResetOTP{
		UserID:    user.ID,
		Email:     email,
		OTP:       "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
		IsUsed:    false,
	}
	s.otpsByEmail[email] = otp
	return otp, nil
}

type fakePasswordResetEmailSender struct {
	sent []sentPasswordResetEmail
}

type sentPasswordResetEmail struct {
	email     string
	otp       string
	firstName string
}

func (s *fakePasswordResetEmailSender) SendPasswordResetEmail(email, otp, firstName string) error {
	s.sent = append(s.sent, sentPasswordResetEmail{
		email:     email,
		otp:       otp,
		firstName: firstName,
	})
	return nil
}

func stubPasswordResetEmailSender(t *testing.T, sender passwordResetEmailSender) {
	t.Helper()

	prev := newPasswordResetEmailSender
	newPasswordResetEmailSender = func(*config.Config) passwordResetEmailSender {
		return sender
	}
	t.Cleanup(func() {
		newPasswordResetEmailSender = prev
	})
}

func (s *fakeAuthService) VerifyPasswordResetOTP(email, otp, newPassword string) (*Auth, error) {
	if s.verifyPasswordResetOTPErr != nil {
		return nil, s.verifyPasswordResetOTPErr
	}
	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	otpRecord, ok := s.otpsByEmail[email]
	if !ok || otpRecord.OTP != otp {
		return nil, ErrInvalidOTP
	}
	if otpRecord.IsUsed {
		return nil, ErrOTPAlreadyUsed
	}
	if time.Now().After(otpRecord.ExpiresAt) {
		return nil, ErrOTPExpired
	}
	otpRecord.IsUsed = true
	return user, nil
}

func (s *fakeAuthService) GetUnusedOTPByEmail(email string) (*PasswordResetOTP, error) {
	otp, ok := s.otpsByEmail[email]
	if !ok || otp.IsUsed || time.Now().After(otp.ExpiresAt) {
		return nil, ErrInvalidOTP
	}
	return otp, nil
}

func setupRouter(service AuthServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, &config.Config{JWTSecret: "test-secret"})
	return r
}

func TestSignUpEndpointCreatesUser(t *testing.T) {
	service := newFakeAuthService()
	router := setupRouter(service)
	body := `{"firstname":"Ada","lastname":"Lovelace","email":"ada@example.com","password":"secret123"}`

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}

	created := service.usersByEmail["ada@example.com"]
	if created == nil {
		t.Fatal("expected user to be created")
	}
	if created.Password == "secret123" {
		t.Fatal("expected password to be hashed")
	}
	if err := util.VerifyPassword("secret123", created.Password); err != nil {
		t.Fatalf("expected stored password hash to verify: %v", err)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	user := payload["user"].(map[string]any)
	if user["email"] != "ada@example.com" {
		t.Fatalf("unexpected email in response: %v", user["email"])
	}
}

func TestSignUpEndpointRejectsInvalidPayload(t *testing.T) {
	router := setupRouter(newFakeAuthService())

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/signup", strings.NewReader(`{"email":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}

	payload := assertAPIError(t, res, http.StatusBadRequest, "validation_error", "Request validation failed")
	if len(payload.Error.Details) == 0 {
		t.Fatal("expected validation details in error response")
	}
}

func TestLoginEndpointReturnsBearerTokens(t *testing.T) {
	service := newFakeAuthService()
	password, err := util.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &Auth{ID: 7, FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com", Password: password, Role: "Admin"}
	service.usersByEmail[user.Email] = user
	service.usersByID[user.ID] = user
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"email":"ada@example.com","password":"secret123","rememberMe":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload struct {
		Message string        `json:"message"`
		Data    LoginResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.AccessToken == "" || payload.Data.RefreshToken == "" {
		t.Fatalf("expected access and refresh tokens, got %#v", payload.Data)
	}
	assertTokenUserID(t, payload.Data.AccessToken, 7)
	assertTokenUserID(t, payload.Data.RefreshToken, 7)
}

func TestLoginEndpointRejectsWrongPassword(t *testing.T) {
	service := newFakeAuthService()
	password, err := util.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &Auth{ID: 7, Email: "ada@example.com", Password: password, Role: "User"}
	service.usersByEmail[user.Email] = user
	service.usersByID[user.ID] = user
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", res.Code)
	}

	assertAPIError(t, res, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
}

func TestSignUpEndpointReturnsConflictForDuplicateEmail(t *testing.T) {
	service := newFakeAuthService()
	service.createErr = ErrEmailAlreadyExists
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/signup", strings.NewReader(`{"firstname":"Ada","lastname":"Lovelace","email":"ada@example.com","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertAPIError(t, res, http.StatusConflict, "conflict", "An account with this email already exists")
}

func TestSignUpEndpointReturnsServiceUnavailableWhenStoreUnavailable(t *testing.T) {
	service := newFakeAuthService()
	service.createErr = ErrStoreUnavailable
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/signup", strings.NewReader(`{"firstname":"Ada","lastname":"Lovelace","email":"ada@example.com","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", res.Code, res.Body.String())
	}

	assertAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Authentication service is temporarily unavailable")
}

func TestLoginEndpointReturnsServiceUnavailableWhenStoreUnavailable(t *testing.T) {
	service := newFakeAuthService()
	service.getUserErr = ErrStoreUnavailable
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"email":"ada@example.com","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", res.Code, res.Body.String())
	}

	assertAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Authentication service is temporarily unavailable")
}

func TestRefreshEndpointReturnsNewAccessToken(t *testing.T) {
	service := newFakeAuthService()
	user := &Auth{ID: 42, FirstName: "Grace", LastName: "Hopper", Email: "grace@example.com", Role: "User"}
	service.usersByEmail[user.Email] = user
	service.usersByID[user.ID] = user

	controller := &AuthController{AuthService: service, CFG: &config.Config{JWTSecret: "test-secret"}}
	refreshToken, err := controller.signToken(user, 24*time.Hour)
	if err != nil {
		t.Fatalf("sign refresh token: %v", err)
	}

	router := setupRouter(service)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload map[string]string
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["accessToken"] == "" {
		t.Fatal("expected accessToken in response")
	}
	assertTokenUserID(t, payload["accessToken"], 42)
}

func TestRefreshEndpointReturnsServiceUnavailableWhenStoreUnavailable(t *testing.T) {
	service := newFakeAuthService()
	service.getUserByIDErr = ErrStoreUnavailable
	user := &Auth{ID: 42, FirstName: "Grace", LastName: "Hopper", Email: "grace@example.com", Role: "User"}
	service.usersByEmail[user.Email] = user
	service.usersByID[user.ID] = user

	controller := &AuthController{AuthService: service, CFG: &config.Config{JWTSecret: "test-secret"}}
	refreshToken, err := controller.signToken(user, 24*time.Hour)
	if err != nil {
		t.Fatalf("sign refresh token: %v", err)
	}

	router := setupRouter(service)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", res.Code, res.Body.String())
	}

	assertAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Authentication service is temporarily unavailable")
}

func TestRefreshEndpointRequiresBearerToken(t *testing.T) {
	router := setupRouter(newFakeAuthService())

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/refresh", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", res.Code)
	}

	assertAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")
}

func TestForgotPasswordEndpointReturnsSuccessAndSendsOTP(t *testing.T) {
	service := newFakeAuthService()
	service.usersByEmail["ada@example.com"] = &Auth{
		ID:        7,
		FirstName: "Ada",
		LastName:  "Lovelace",
		Email:     "ada@example.com",
		Role:      "User",
	}
	emailSender := &fakePasswordResetEmailSender{}
	stubPasswordResetEmailSender(t, emailSender)

	router := setupRouter(service)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/forgot-password", strings.NewReader(`{"email":"ada@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload map[string]string
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != passwordResetSentMessage {
		t.Fatalf("unexpected response payload: %#v", payload)
	}

	if len(emailSender.sent) != 1 {
		t.Fatalf("expected one password reset email to be sent, got %#v", emailSender.sent)
	}
	if emailSender.sent[0].email != "ada@example.com" || emailSender.sent[0].otp != "123456" || emailSender.sent[0].firstName != "Ada" {
		t.Fatalf("unexpected email send payload: %#v", emailSender.sent[0])
	}
}

func TestForgotPasswordEndpointReturnsSuccessForUnknownUser(t *testing.T) {
	emailSender := &fakePasswordResetEmailSender{}
	stubPasswordResetEmailSender(t, emailSender)

	router := setupRouter(newFakeAuthService())
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/forgot-password", strings.NewReader(`{"email":"missing@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload map[string]string
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != passwordResetSentMessage {
		t.Fatalf("unexpected response payload: %#v", payload)
	}
	if len(emailSender.sent) != 0 {
		t.Fatalf("expected no email to be sent for unknown user, got %#v", emailSender.sent)
	}
}

func TestForgotPasswordEndpointRejectsInvalidPayload(t *testing.T) {
	router := setupRouter(newFakeAuthService())

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/forgot-password", strings.NewReader(`{"email":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	payload := assertAPIError(t, res, http.StatusBadRequest, "validation_error", "Request validation failed")
	if len(payload.Error.Details) == 0 {
		t.Fatal("expected validation details in error response")
	}
}

func TestForgotPasswordEndpointReturnsServiceUnavailableWhenStoreUnavailable(t *testing.T) {
	service := newFakeAuthService()
	service.createPasswordResetOTPErr = ErrStoreUnavailable
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/forgot-password", strings.NewReader(`{"email":"ada@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Authentication service is temporarily unavailable")
}

func TestForgotPasswordEndpointReturnsInternalErrorForUnexpectedFailure(t *testing.T) {
	service := newFakeAuthService()
	service.createPasswordResetOTPErr = errors.New("boom")
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/forgot-password", strings.NewReader(`{"email":"ada@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertAPIError(t, res, http.StatusInternalServerError, "internal_error", "Internal server error")
}

func TestResetPasswordEndpointResetsPassword(t *testing.T) {
	service := newFakeAuthService()
	service.usersByEmail["ada@example.com"] = &Auth{
		ID:        7,
		FirstName: "Ada",
		LastName:  "Lovelace",
		Email:     "ada@example.com",
		Role:      "Admin",
	}
	service.otpsByEmail["ada@example.com"] = &PasswordResetOTP{
		UserID:    7,
		Email:     "ada@example.com",
		OTP:       "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/reset-password", strings.NewReader(`{"email":"ada@example.com","otp":"123456","password":"newSecret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	user := payload["user"].(map[string]any)
	if payload["message"] != "Password reset successfully" || user["email"] != "ada@example.com" || user["role"] != "Admin" {
		t.Fatalf("unexpected response payload: %#v", payload)
	}
}

func TestResetPasswordEndpointRejectsInvalidPayload(t *testing.T) {
	router := setupRouter(newFakeAuthService())

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/reset-password", strings.NewReader(`{"email":"ada@example.com","otp":"123","password":"short"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	payload := assertAPIError(t, res, http.StatusBadRequest, "validation_error", "Request validation failed")
	if len(payload.Error.Details) == 0 {
		t.Fatal("expected validation details in error response")
	}
}

func TestResetPasswordEndpointReturnsValidationErrorsForOTPFailures(t *testing.T) {
	tests := []struct {
		name        string
		serviceErr  error
		wantMessage string
	}{
		{name: "invalid otp", serviceErr: ErrInvalidOTP, wantMessage: "Invalid OTP"},
		{name: "used otp", serviceErr: ErrOTPAlreadyUsed, wantMessage: "OTP has already been used. Please request a new OTP"},
		{name: "expired otp", serviceErr: ErrOTPExpired, wantMessage: "OTP has expired. Please request a new OTP"},
		{name: "missing user", serviceErr: ErrUserNotFound, wantMessage: "Invalid OTP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newFakeAuthService()
			service.verifyPasswordResetOTPErr = tt.serviceErr
			router := setupRouter(service)

			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/user/reset-password", strings.NewReader(`{"email":"ada@example.com","otp":"123456","password":"newSecret123"}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(res, req)

			assertAPIError(t, res, http.StatusBadRequest, "validation_error", tt.wantMessage)
		})
	}
}

func TestResetPasswordEndpointReturnsServiceUnavailableWhenStoreUnavailable(t *testing.T) {
	service := newFakeAuthService()
	service.verifyPasswordResetOTPErr = ErrStoreUnavailable
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/reset-password", strings.NewReader(`{"email":"ada@example.com","otp":"123456","password":"newSecret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Authentication service is temporarily unavailable")
}

func TestResetPasswordEndpointReturnsInternalErrorForUnexpectedFailure(t *testing.T) {
	service := newFakeAuthService()
	service.verifyPasswordResetOTPErr = errors.New("boom")
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/reset-password", strings.NewReader(`{"email":"ada@example.com","otp":"123456","password":"newSecret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertAPIError(t, res, http.StatusInternalServerError, "internal_error", "Internal server error")
}

func TestBearerToken(t *testing.T) {
	token, err := bearerToken("Bearer abc.def")
	if err != nil {
		t.Fatalf("expected bearer token, got error: %v", err)
	}
	if token != "abc.def" {
		t.Fatalf("unexpected token: %q", token)
	}

	if _, err := bearerToken("Basic abc.def"); err == nil {
		t.Fatal("expected non-bearer header to fail")
	}
}

func TestClaimInt(t *testing.T) {
	if got, ok := claimInt(float64(12)); !ok || got != 12 {
		t.Fatalf("expected float64 claim to become 12, got %d ok=%v", got, ok)
	}
	if got, ok := claimInt(13); !ok || got != 13 {
		t.Fatalf("expected int claim to become 13, got %d ok=%v", got, ok)
	}
	if _, ok := claimInt("14"); ok {
		t.Fatal("expected string claim to be rejected")
	}
}

func assertTokenUserID(t *testing.T, tokenString string, want int) {
	t.Helper()

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		t.Fatalf("expected valid map claims, got valid=%v claims=%T", token.Valid, token.Claims)
	}

	got, ok := claimInt(claims["user_id"])
	if !ok || got != want {
		t.Fatalf("expected user_id %d, got %v ok=%v", want, claims["user_id"], ok)
	}
}

func assertAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
	t.Helper()

	if res.Code != wantStatus {
		t.Fatalf("expected status %d, got %d: %s", wantStatus, res.Code, res.Body.String())
	}

	var payload apiresponse.ErrorResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != wantCode {
		t.Fatalf("expected error code %q, got %#v", wantCode, payload)
	}
	if payload.Error.Message != wantMessage {
		t.Fatalf("expected error message %q, got %#v", wantMessage, payload)
	}
	return payload
}

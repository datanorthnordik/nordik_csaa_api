package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nordikcsaaapi/internal/config"
	"nordikcsaaapi/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type fakeAuthService struct {
	usersByEmail map[string]*Auth
	usersByID    map[int]*Auth
	createErr    error
}

func newFakeAuthService() *fakeAuthService {
	return &fakeAuthService{
		usersByEmail: map[string]*Auth{},
		usersByID:    map[int]*Auth{},
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
	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return user, nil
}

func (s *fakeAuthService) GetUserByID(id int) (*Auth, error) {
	user, ok := s.usersByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return user, nil
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

func TestRefreshEndpointRequiresBearerToken(t *testing.T) {
	router := setupRouter(newFakeAuthService())

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/refresh", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", res.Code)
	}
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

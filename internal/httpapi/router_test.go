package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/config"
)

func TestSampleEndpoint(t *testing.T) {
	router := NewRouter(config.Load(), slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sample", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["message"] != "sample endpoint works" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestLoginPlaceholder(t *testing.T) {
	router := NewRouter(config.Load(), slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"demo@nordik.local"}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["message"] != "login api works for now" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

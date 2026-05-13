package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
)

func TestRequireAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouter := func(cfg *config.Config) *gin.Engine {
		r := gin.New()
		r.POST("/protected", RequireAPIKey(cfg), func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
		return r
	}

	t.Run("accepts matching api key", func(t *testing.T) {
		router := newRouter(&config.Config{APIKey: "test-api-key"})

		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/protected", nil)
		req.Header.Set("X-API-Key", "test-api-key")
		router.ServeHTTP(res, req)

		if res.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d: %s", res.Code, res.Body.String())
		}
	})

	t.Run("rejects missing api key", func(t *testing.T) {
		router := newRouter(&config.Config{APIKey: "test-api-key"})

		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/protected", nil)
		router.ServeHTTP(res, req)

		assertAPIError(t, res, http.StatusUnauthorized, "missing_api_key", "Missing API key")
	})

	t.Run("rejects invalid api key", func(t *testing.T) {
		router := newRouter(&config.Config{APIKey: "test-api-key"})

		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/protected", nil)
		req.Header.Set("X-API-Key", "wrong-key")
		router.ServeHTTP(res, req)

		assertAPIError(t, res, http.StatusUnauthorized, "invalid_api_key", "Invalid API key")
	})

	t.Run("fails when api key is not configured", func(t *testing.T) {
		router := newRouter(&config.Config{})

		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/protected", nil)
		req.Header.Set("X-API-Key", "test-api-key")
		router.ServeHTTP(res, req)

		assertAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "API key authentication is temporarily unavailable")
	})
}

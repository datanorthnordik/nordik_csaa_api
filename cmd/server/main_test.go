package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func TestBuildCORSConfigAllowsPatchRequests(t *testing.T) {
	config := buildCORSConfig()

	if !slices.Contains(config.AllowMethods, http.MethodPatch) {
		t.Fatalf("expected CORS config to allow %s, got %v", http.MethodPatch, config.AllowMethods)
	}
}

func TestBuildCORSConfigAllowsConfiguredCloudRunOrigins(t *testing.T) {
	t.Setenv("GIN_MODE", gin.TestMode)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(cors.New(buildCORSConfig()))
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	testCases := []struct {
		name           string
		origin         string
		expectedStatus int
		expectedHeader string
	}{
		{
			name:           "website ui origin",
			origin:         "https://nordikcsaawebsiteui-724838782318.us-west1.run.app",
			expectedStatus: http.StatusOK,
			expectedHeader: "https://nordikcsaawebsiteui-724838782318.us-west1.run.app",
		},
		{
			name:           "cms ui origin",
			origin:         "https://nordikcsaacmsui-724838782318.us-west1.run.app",
			expectedStatus: http.StatusOK,
			expectedHeader: "https://nordikcsaacmsui-724838782318.us-west1.run.app",
		},
		{
			name:           "other project blocked",
			origin:         "https://nordikcsaawebsiteui-999999999999.us-west1.run.app",
			expectedStatus: http.StatusForbidden,
			expectedHeader: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.Header.Set("Origin", tc.origin)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tc.expectedHeader {
				t.Fatalf("expected Access-Control-Allow-Origin %q, got %q", tc.expectedHeader, got)
			}
		})
	}
}

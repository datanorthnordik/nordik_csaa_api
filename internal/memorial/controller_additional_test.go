package memorial

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMemorialControllerAdditionalBranches(t *testing.T) {
	t.Run("nil service returns internal error across endpoints", func(t *testing.T) {
		router := setupMemorialRouter(nil)

		tests := []struct {
			method string
			path   string
			body   string
		}{
			{method: http.MethodGet, path: "/api/memorial/7"},
			{method: http.MethodGet, path: "/api/memorial/7/portrait/content"},
			{method: http.MethodGet, path: "/api/memorial/7/gallery/3/content"},
			{method: http.MethodPost, path: "/api/memorial", body: `{"full_name":"Ada","category":"founder","status":"draft"}`},
			{method: http.MethodPut, path: "/api/memorial/7", body: `{"full_name":"Ada","category":"founder","status":"draft"}`},
			{method: http.MethodDelete, path: "/api/memorial/7"},
		}

		for _, tc := range tests {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(res, req)
			assertMemorialAPIError(t, res, http.StatusInternalServerError, "internal_error", "Internal server error")
		}
	})

	t.Run("invalid gallery media id returns validation error", func(t *testing.T) {
		router := setupMemorialRouter(&fakeMemorialService{})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/memorial/7/gallery/bad/content", nil)
		router.ServeHTTP(res, req)

		payload := assertMemorialAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
		if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "mediaId" {
			t.Fatalf("unexpected error details: %#v", payload)
		}
	})

	t.Run("media endpoints omit content disposition when filename is blank", func(t *testing.T) {
		service := &fakeMemorialService{
			portraitResp: &MemorialMediaContent{
				Content: []byte("portrait-bits"),
			},
			galleryResp: &MemorialMediaContent{
				Content: []byte("gallery-bits"),
			},
		}
		router := setupMemorialRouter(service)

		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/memorial/7/portrait/content", nil)
		router.ServeHTTP(res, req)
		if got := res.Header().Get("Content-Disposition"); got != "" {
			t.Fatalf("expected no content disposition header, got %q", got)
		}
		if got := res.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("expected default content type, got %q", got)
		}

		res = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/memorial/7/gallery/3/content", nil)
		router.ServeHTTP(res, req)
		if got := res.Header().Get("Content-Disposition"); got != "" {
			t.Fatalf("expected no content disposition header, got %q", got)
		}
		if got := res.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("expected default content type, got %q", got)
		}
	})

	t.Run("client safe error helper handles nil and supported messages", func(t *testing.T) {
		if isClientSafeMemorialError(nil) {
			t.Fatal("expected nil errors to be unsafe")
		}
		if !isClientSafeMemorialError(errors.New("use multipart/form-data with a payload field for file uploads")) {
			t.Fatal("expected multipart usage errors to be treated as client-safe")
		}
		if !isClientSafeMemorialError(errors.New("memorial content is not available from storage")) {
			t.Fatal("expected storage availability message to be client-safe")
		}
	})
}

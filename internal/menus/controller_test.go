package menus

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/apiresponse"
	authpkg "nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
)

type fakeMenuService struct {
	getResp         *MenuResponse
	getErr          error
	pageOptionsResp *MenuPageOptionsResponse
	pageOptionsErr  error
	saveResp        *MenuResponse
	saveErr         error
	gotKey          string
	gotSaveKey      string
	gotSaveReq      SaveMenuRequest
}

func (s *fakeMenuService) GetMenu(key string) (*MenuResponse, error) {
	s.gotKey = key
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getResp == nil {
		return &MenuResponse{MenuKey: key, Name: "Main Website Navigation", Items: []MenuItemResponse{}}, nil
	}
	return s.getResp, nil
}

func (s *fakeMenuService) ListMenuPageOptions() (*MenuPageOptionsResponse, error) {
	if s.pageOptionsErr != nil {
		return nil, s.pageOptionsErr
	}
	if s.pageOptionsResp == nil {
		return &MenuPageOptionsResponse{Items: []MenuPageOption{}}, nil
	}
	return s.pageOptionsResp, nil
}

func (s *fakeMenuService) SaveMenu(key string, req SaveMenuRequest) (*MenuResponse, error) {
	s.gotSaveKey = key
	s.gotSaveReq = req
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	if s.saveResp == nil {
		return &MenuResponse{MenuKey: key, Name: req.Name, Items: []MenuItemResponse{}}, nil
	}
	return s.saveResp, nil
}

func setupMenuRouter(service MenuServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service)
	return r
}

func setupProtectedMenuRouter(service MenuServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireAPIKey(&config.Config{APIKey: "test-api-key"}))
	return r
}

func TestGetMenuEndpoint(t *testing.T) {
	service := &fakeMenuService{
		getResp: &MenuResponse{
			ID:      1,
			MenuKey: "main",
			Name:    "Main Website Navigation",
			Items:   []MenuItemResponse{{ID: 10, Label: "About", NavigationType: NavigationTypePage}},
		},
	}
	router := setupMenuRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/menus/main", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotKey != "main" {
		t.Fatalf("expected key main, got %q", service.gotKey)
	}
}

func TestListMenuPageOptionsAndSaveMenuEndpoints(t *testing.T) {
	service := &fakeMenuService{
		pageOptionsResp: &MenuPageOptionsResponse{
			Items: []MenuPageOption{{ID: 10, PageTitle: "About Us", URLSlug: "/about", Status: "published"}},
		},
	}
	router := setupProtectedMenuRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/menus/main/page-options", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	body := `{"name":"Main Website Navigation","items":[{"label":"About","navigation_type":"pages","page_id":10,"children":[]}]}`
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/menus/main", strings.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotSaveKey != "main" {
		t.Fatalf("expected save key main, got %q", service.gotSaveKey)
	}
	if service.gotSaveReq.UpdatedBy != nil {
		t.Fatalf("expected API key writes to omit updated_by, got %#v", service.gotSaveReq)
	}
	if len(service.gotSaveReq.Items) != 1 || service.gotSaveReq.Items[0].PageID == nil || *service.gotSaveReq.Items[0].PageID != 10 {
		t.Fatalf("unexpected save payload: %#v", service.gotSaveReq)
	}
}

func TestMenuEndpointErrorsAndProtection(t *testing.T) {
	router := setupMenuRouter(&fakeMenuService{getErr: ErrStoreUnavailable})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/menus/main", nil)
	router.ServeHTTP(res, req)
	assertMenuAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Menu service is temporarily unavailable")

	protected := setupProtectedMenuRouter(&fakeMenuService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/menus/main", strings.NewReader(`{"name":"Main Website Navigation","items":[]}`))
	req.Header.Set("Content-Type", "application/json")
	protected.ServeHTTP(res, req)
	assertMenuAPIError(t, res, http.StatusUnauthorized, "missing_api_key", "Missing API key")
}

func TestMenuEndpointAdditionalErrorsAndHelpers(t *testing.T) {
	t.Run("get menu not found", func(t *testing.T) {
		router := setupMenuRouter(&fakeMenuService{getErr: ErrMenuNotFound})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/menus/main", nil)
		router.ServeHTTP(res, req)
		assertMenuAPIError(t, res, http.StatusNotFound, "not_found", "menu not found")
	})

	t.Run("page options service unavailable", func(t *testing.T) {
		router := setupProtectedMenuRouter(&fakeMenuService{pageOptionsErr: ErrStoreUnavailable})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/menus/main/page-options", nil)
		router.ServeHTTP(res, req)
		assertMenuAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Menu service is temporarily unavailable")
	})

	t.Run("save invalid json payload", func(t *testing.T) {
		router := setupProtectedMenuRouter(&fakeMenuService{})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/menus/main", strings.NewReader(`{"name":`))
		req.Header.Set("X-API-Key", "test-api-key")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMenuAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid request payload")
	})

	t.Run("save validation error", func(t *testing.T) {
		router := setupProtectedMenuRouter(&fakeMenuService{saveErr: errors.New("page_id is required when navigation_type is pages")})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/menus/main", strings.NewReader(`{"name":"Main","items":[]}`))
		req.Header.Set("X-API-Key", "test-api-key")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMenuAPIError(t, res, http.StatusBadRequest, "validation_error", "page_id is required when navigation_type is pages")
	})

	t.Run("save conflict error", func(t *testing.T) {
		router := setupProtectedMenuRouter(&fakeMenuService{saveErr: errors.New(`ERROR: duplicate key value violates unique constraint "uq_menu_items_page_per_menu" (SQLSTATE 23505)`)})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/menus/main", strings.NewReader(`{"name":"Main","items":[]}`))
		req.Header.Set("X-API-Key", "test-api-key")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMenuAPIError(t, res, http.StatusConflict, "conflict", "Unable to save menu because a conflicting record already exists")
	})

	t.Run("save unexpected error", func(t *testing.T) {
		router := setupProtectedMenuRouter(&fakeMenuService{saveErr: errors.New("database timed out")})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/menus/main", strings.NewReader(`{"name":"Main","items":[]}`))
		req.Header.Set("X-API-Key", "test-api-key")
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMenuAPIError(t, res, http.StatusInternalServerError, "internal_error", "Internal server error")
	})

	t.Run("client safe error matcher", func(t *testing.T) {
		cases := []struct {
			err  error
			want bool
		}{
			{err: nil, want: false},
			{err: errors.New("page_id is required when navigation_type is pages"), want: true},
			{err: errors.New("invalid navigation_type"), want: true},
			{err: errors.New("external_url must be a valid URL"), want: true},
			{err: errors.New("database timeout"), want: false},
		}

		for _, tc := range cases {
			if got := isClientSafeMenuError(tc.err); got != tc.want {
				t.Fatalf("isClientSafeMenuError(%v)=%v want %v", tc.err, got, tc.want)
			}
		}
	})

	t.Run("auth user id helper", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)

		if got := authUserIDFromContext(c); got != nil {
			t.Fatalf("expected nil auth user id, got %#v", got)
		}

		c.Set("auth_user_id", 4)
		if got := authUserIDFromContext(c); got == nil || *got != 4 {
			t.Fatalf("expected int auth user id 4, got %#v", got)
		}
		c.Set("auth_user_id", int32(5))
		if got := authUserIDFromContext(c); got == nil || *got != 5 {
			t.Fatalf("expected int32 auth user id 5, got %#v", got)
		}
		c.Set("auth_user_id", int64(6))
		if got := authUserIDFromContext(c); got == nil || *got != 6 {
			t.Fatalf("expected int64 auth user id 6, got %#v", got)
		}
		c.Set("auth_user_id", "bad")
		if got := authUserIDFromContext(c); got != nil {
			t.Fatalf("expected nil auth user id for unsupported type, got %#v", got)
		}
	})

	t.Run("path int helper", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Params = gin.Params{{Key: "id", Value: "12"}}
		if got, ok := pathInt(c, "id"); !ok || got != 12 {
			t.Fatalf("expected valid path int 12, got %d ok=%v", got, ok)
		}

		rec = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(rec)
		c.Params = gin.Params{{Key: "id", Value: "bad"}}
		if _, ok := pathInt(c, "id"); ok {
			t.Fatal("expected invalid path int to fail")
		}
		payload := assertMenuAPIError(t, rec, http.StatusBadRequest, "validation_error", "Invalid path parameter")
		if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
			t.Fatalf("expected id path detail, got %#v", payload)
		}
	})
}

func assertMenuAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

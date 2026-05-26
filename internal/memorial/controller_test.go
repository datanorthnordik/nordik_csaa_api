package memorial

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nordikcsaaapi/internal/apiresponse"
	authpkg "nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type fakeMemorialService struct {
	listResp                *MemorialListResponse
	detailResp              *MemorialDetailResponse
	portraitResp            *MemorialMediaContent
	galleryResp             *MemorialMediaContent
	mutationResp            *MemorialMutationResponse
	listErr                 error
	detailErr               error
	portraitErr             error
	galleryErr              error
	createErr               error
	updateErr               error
	deleteErr               error
	gotListFilter           ListMemorialsFilter
	gotGetID                int
	gotPortraitID           int
	gotGalleryID            int
	gotGalleryMediaID       int
	gotCreateRequest        SaveMemorialRequest
	gotCreateUserID         *int
	gotUpdateID             int
	gotUpdateRequest        SaveMemorialRequest
	gotUpdateUserID         *int
	gotDeleteID             int
}

func cloneMemorialUserID(userID *int) *int {
	if userID == nil {
		return nil
	}
	cloned := *userID
	return &cloned
}

func (s *fakeMemorialService) ListMemorials(filter ListMemorialsFilter) (*MemorialListResponse, error) {
	s.gotListFilter = filter
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResp == nil {
		return &MemorialListResponse{}, nil
	}
	return s.listResp, nil
}

func (s *fakeMemorialService) GetMemorial(id int) (*MemorialDetailResponse, error) {
	s.gotGetID = id
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	if s.detailResp == nil {
		return &MemorialDetailResponse{ID: id, FullName: "Ada Lovelace"}, nil
	}
	return s.detailResp, nil
}

func (s *fakeMemorialService) GetMemorialPortraitContent(id int) (*MemorialMediaContent, error) {
	s.gotPortraitID = id
	if s.portraitErr != nil {
		return nil, s.portraitErr
	}
	if s.portraitResp == nil {
		return &MemorialMediaContent{
			Content:     []byte("portrait"),
			ContentType: "image/jpeg",
			FileName:    "portrait.jpg",
		}, nil
	}
	return s.portraitResp, nil
}

func (s *fakeMemorialService) GetMemorialGalleryImageContent(id int, mediaID int) (*MemorialMediaContent, error) {
	s.gotGalleryID = id
	s.gotGalleryMediaID = mediaID
	if s.galleryErr != nil {
		return nil, s.galleryErr
	}
	if s.galleryResp == nil {
		return &MemorialMediaContent{
			Content:     []byte("gallery"),
			ContentType: "image/png",
			FileName:    "gallery.png",
		}, nil
	}
	return s.galleryResp, nil
}

func (s *fakeMemorialService) CreateMemorial(req SaveMemorialRequest, userID *int) (*MemorialMutationResponse, error) {
	s.gotCreateRequest = req
	s.gotCreateUserID = cloneMemorialUserID(userID)
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.mutationResp == nil {
		return &MemorialMutationResponse{ID: 7, FullName: req.FullName, Category: req.Category, Status: req.Status}, nil
	}
	return s.mutationResp, nil
}

func (s *fakeMemorialService) UpdateMemorial(id int, req SaveMemorialRequest, userID *int) (*MemorialMutationResponse, error) {
	s.gotUpdateID = id
	s.gotUpdateRequest = req
	s.gotUpdateUserID = cloneMemorialUserID(userID)
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.mutationResp == nil {
		return &MemorialMutationResponse{ID: id, FullName: req.FullName, Category: req.Category, Status: req.Status}, nil
	}
	return s.mutationResp, nil
}

func (s *fakeMemorialService) DeleteMemorial(id int) error {
	s.gotDeleteID = id
	return s.deleteErr
}

func setupMemorialRouter(service MemorialServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service)
	return r
}

func setupProtectedMemorialRouter(service MemorialServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func signMemorialTestToken(t *testing.T, secret string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": 7,
		"email":   "ada@example.com",
		"role":    "Admin",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func assertMemorialAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

func TestMemorialListDetailAndMediaEndpoints(t *testing.T) {
	service := &fakeMemorialService{
		listResp: &MemorialListResponse{
			Items: []MemorialListItem{
				{
					ID:            9,
					FullName:      "Ada Lovelace",
					Affiliation:   "Analytical Engine",
					Category:      MemorialCategoryFounder,
					CategoryLabel: "Founder",
					Status:        MemorialStatusPublished,
				},
			},
			Pagination: MemorialListPageMeta{Page: 2, PageSize: 5, TotalItems: 9, TotalPages: 2, HasPrev: true},
		},
		detailResp: &MemorialDetailResponse{
			ID:            9,
			FullName:      "Ada Lovelace",
			Affiliation:   "Analytical Engine",
			Category:      MemorialCategoryFounder,
			CategoryLabel: "Founder",
			Status:        MemorialStatusPublished,
			Biography:     "<p>First programmer</p>",
			GalleryImages: []MemorialGalleryImageResponse{{ID: 3, FileName: "gallery.png"}},
		},
		portraitResp: &MemorialMediaContent{
			Content:  []byte("portrait-bits"),
			FileName: "portrait\r\nfinal.jpg",
		},
		galleryResp: &MemorialMediaContent{
			Content:     []byte("gallery-bits"),
			ContentType: "image/png",
			FileName:    "gallery.png",
		},
	}
	router := setupMemorialRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/memorial?page=2&page_size=5&search=Ada&status=published&category=founder&sort_by=full_name&sort_order=asc", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter != (ListMemorialsFilter{
		Page:       2,
		PageSize:   5,
		SearchTerm: "Ada",
		Status:     "published",
		Category:   "founder",
		SortBy:     "full_name",
		SortOrder:  "asc",
		PublicOnly: true,
	}) {
		t.Fatalf("unexpected filter: %#v", service.gotListFilter)
	}

	var listPayload MemorialListResponse
	if err := json.NewDecoder(res.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].FullName != "Ada Lovelace" {
		t.Fatalf("unexpected list payload: %#v", listPayload)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memorial/9", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotGetID != 9 {
		t.Fatalf("expected detail id 9, got %d", service.gotGetID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memorial/9/portrait/content", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected portrait status 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("expected default content type, got %q", got)
	}
	if got := res.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="portraitfinal.jpg"`) {
		t.Fatalf("expected sanitized filename header, got %q", got)
	}
	if service.gotPortraitID != 9 {
		t.Fatalf("expected portrait id 9, got %d", service.gotPortraitID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/memorial/9/gallery/3/content", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected gallery status 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png content type, got %q", got)
	}
	if service.gotGalleryID != 9 || service.gotGalleryMediaID != 3 {
		t.Fatalf("unexpected gallery args: id=%d media=%d", service.gotGalleryID, service.gotGalleryMediaID)
	}
}

func TestMemorialProtectedWriteEndpointsAndAuth(t *testing.T) {
	router := setupProtectedMemorialRouter(&fakeMemorialService{})

	for _, path := range []string{
		"/api/memorial",
		"/api/memorial/7",
		"/api/memorial/7/portrait/content",
		"/api/memorial/7/gallery/3/content",
	} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected public read endpoint %s to be accessible without auth, got %d: %s", path, res.Code, res.Body.String())
		}
	}

	tests := []struct {
		method string
		path   string
		body   string
	}{
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
		assertMemorialAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")
	}

	service := &fakeMemorialService{
		mutationResp: &MemorialMutationResponse{ID: 7, FullName: "Ada", Category: MemorialCategoryFounder, Status: MemorialStatusDraft},
	}
	router = setupProtectedMemorialRouter(service)
	token := signMemorialTestToken(t, "test-secret")

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/memorial", strings.NewReader(`{"full_name":"Ada","category":"founder","status":"draft","biography":"<p>Hello</p>"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateUserID == nil || *service.gotCreateUserID != 7 {
		t.Fatalf("expected auth user id 7, got %#v", service.gotCreateUserID)
	}
	if service.gotCreateRequest.FullName != "Ada" {
		t.Fatalf("unexpected create request: %#v", service.gotCreateRequest)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/memorial/7", strings.NewReader(`{"full_name":"Ada Updated","category":"friend","status":"review"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 7 {
		t.Fatalf("expected update id 7, got %d", service.gotUpdateID)
	}
	if service.gotUpdateUserID == nil || *service.gotUpdateUserID != 7 {
		t.Fatalf("expected update auth user id 7, got %#v", service.gotUpdateUserID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/memorial/7", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDeleteID != 7 {
		t.Fatalf("expected delete id 7, got %d", service.gotDeleteID)
	}
}

func TestMemorialControllerErrorsAndHelpers(t *testing.T) {
	t.Run("nil service returns internal error", func(t *testing.T) {
		router := setupMemorialRouter(nil)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/memorial", nil)
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusInternalServerError, "internal_error", "Internal server error")
	})

	t.Run("invalid path id returns validation error", func(t *testing.T) {
		router := setupMemorialRouter(&fakeMemorialService{})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/memorial/bad", nil)
		router.ServeHTTP(res, req)
		payload := assertMemorialAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
		if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
			t.Fatalf("unexpected error details: %#v", payload)
		}
	})

	t.Run("write endpoint bind and service errors", func(t *testing.T) {
		service := &fakeMemorialService{
			createErr: ErrStoreUnavailable,
			updateErr: ErrMemorialNotFound,
			deleteErr: ErrMemorialNotFound,
		}
		router := setupProtectedMemorialRouter(service)
		token := signMemorialTestToken(t, "test-secret")

		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/memorial", strings.NewReader(`{"full_name":`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")

		res = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/api/memorial", strings.NewReader(`{"full_name":"Ada","category":"friend","status":"draft"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Memorial service is temporarily unavailable")

		res = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/api/memorial/bad", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")

		res = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/api/memorial/7", strings.NewReader(`{"full_name":"Ada","category":"friend","status":"draft"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusNotFound, "not_found", ErrMemorialNotFound.Error())

		res = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/api/memorial/7", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusNotFound, "not_found", ErrMemorialNotFound.Error())
	})

	t.Run("read endpoint service errors", func(t *testing.T) {
		service := &fakeMemorialService{
			detailErr:   ErrMemorialNotFound,
			portraitErr: ErrMemorialMediaNotFound,
			galleryErr:  ErrMemorialMediaNotFound,
		}
		router := setupMemorialRouter(service)

		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/memorial/7", nil)
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusNotFound, "not_found", ErrMemorialNotFound.Error())

		res = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/memorial/7/portrait/content", nil)
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusNotFound, "not_found", ErrMemorialMediaNotFound.Error())

		res = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/memorial/7/gallery/3/content", nil)
		router.ServeHTTP(res, req)
		assertMemorialAPIError(t, res, http.StatusNotFound, "not_found", ErrMemorialMediaNotFound.Error())
	})

	t.Run("write memorial error maps expected status codes", func(t *testing.T) {
		tests := []struct {
			name        string
			err         error
			wantStatus  int
			wantCode    string
			wantMessage string
		}{
			{
				name:        "service unavailable",
				err:         ErrStoreUnavailable,
				wantStatus:  http.StatusServiceUnavailable,
				wantCode:    "service_unavailable",
				wantMessage: "Memorial service is temporarily unavailable",
			},
			{
				name:        "not found",
				err:         ErrMemorialNotFound,
				wantStatus:  http.StatusNotFound,
				wantCode:    "not_found",
				wantMessage: ErrMemorialNotFound.Error(),
			},
			{
				name:        "conflict",
				err:         errors.New("duplicate key value violates unique constraint"),
				wantStatus:  http.StatusConflict,
				wantCode:    "conflict",
				wantMessage: "Unable to save memorial entry because a conflicting record already exists",
			},
			{
				name:        "validation",
				err:         errors.New("full_name is required"),
				wantStatus:  http.StatusBadRequest,
				wantCode:    "validation_error",
				wantMessage: "full_name is required",
			},
			{
				name:        "fallback internal error",
				err:         errors.New("boom"),
				wantStatus:  http.StatusInternalServerError,
				wantCode:    "internal_error",
				wantMessage: "Internal server error",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodGet, "/api/memorial", nil)
				writeMemorialError(c, tc.err)
				assertMemorialAPIError(t, rec, tc.wantStatus, tc.wantCode, tc.wantMessage)
			})
		}
	})

	t.Run("controller helpers", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodGet, "/api/memorial?page=2&page_size=999", nil)
		c.Request = req
		if got := queryInt(c, "page", 1, 1, 0); got != 2 {
			t.Fatalf("expected parsed page value, got %d", got)
		}
		if got := queryInt(c, "page_size", 10, 1, 100); got != 10 {
			t.Fatalf("expected fallback page size, got %d", got)
		}

		c.Params = gin.Params{{Key: "id", Value: "4"}}
		if got, ok := pathInt(c, "id"); !ok || got != 4 {
			t.Fatalf("expected valid path int, got %d ok=%v", got, ok)
		}

		for _, tc := range []struct {
			value any
			want  int
		}{
			{value: 7, want: 7},
			{value: int32(8), want: 8},
			{value: int64(9), want: 9},
			{value: uint(10), want: 10},
			{value: float64(11), want: 11},
			{value: "12", want: 12},
		} {
			ctx := &gin.Context{}
			ctx.Set("auth_user_id", tc.value)
			got := authUserID(ctx)
			if got == nil || *got != tc.want {
				t.Fatalf("expected auth user id %d from %#v, got %#v", tc.want, tc.value, got)
			}
		}
		if got := authUserID(&gin.Context{}); got != nil {
			t.Fatalf("expected nil auth user id, got %#v", got)
		}

		if got := sanitizeContentDispositionFilename(` portrait/"final";.jpg `); got != "portraitfinal.jpg" {
			t.Fatalf("unexpected sanitized filename: %q", got)
		}
		if !isClientSafeMemorialError(errors.New("category must be one of alumnus, veteran, founder, friend")) {
			t.Fatal("expected validation message to be client-safe")
		}
		if isClientSafeMemorialError(errors.New("database offline")) {
			t.Fatal("expected generic error to be treated as unsafe")
		}

		handlers := withProtected(func(*gin.Context) {}, func(*gin.Context) {}, func(*gin.Context) {})
		if len(handlers) != 3 {
			t.Fatalf("expected protected handlers plus action, got %d", len(handlers))
		}
	})
}

package press

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"nordikcsaaapi/internal/apiresponse"
	authpkg "nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type fakePressService struct {
	listResp            *PressListResponse
	detailResp          *PressDetailResponse
	mediaContentResp    *PressMediaContent
	createResp          *PressMutationResponse
	updateResp          *PressMutationResponse
	addMediaResp        *AddPressMediaResponse
	updateMediaResp     *PressMediaResponse
	reorderResp         *ReorderPressMediaResponse
	deleteMediaResp     *DeletePressMediaResponse
	listErr             error
	detailErr           error
	mediaContentErr     error
	createErr           error
	updateErr           error
	deleteErr           error
	addMediaErr         error
	updateMediaErr      error
	reorderErr          error
	deleteMediaErr      error
	gotListFilter       ListPressFilter
	gotDetailID         int
	gotMediaEntryID     int
	gotMediaID          int
	gotCreateReq        SavePressEntryRequest
	gotCreateUserID     *int
	gotUpdateID         int
	gotUpdateReq        SavePressEntryRequest
	gotUpdateUserID     *int
	gotDeleteID         int
	gotAddMediaID       int
	gotAddMediaReq      AddPressMediaRequest
	gotAddMediaUserID   *int
	gotUpdateMediaID    int
	gotUpdateMediaMedia int
	gotUpdateMediaReq   UpdatePressMediaRequest
	gotReorderID        int
	gotReorderMediaIDs  []int
	gotDeleteMediaID    int
	gotDeleteMediaIDs   []int
}

func (f *fakePressService) ListPressEntries(filter ListPressFilter) (*PressListResponse, error) {
	f.gotListFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp == nil {
		return &PressListResponse{}, nil
	}
	return f.listResp, nil
}

func (f *fakePressService) GetPressEntry(id int) (*PressDetailResponse, error) {
	f.gotDetailID = id
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if f.detailResp == nil {
		return &PressDetailResponse{ID: id, Title: "Spring Fair"}, nil
	}
	return f.detailResp, nil
}

func (f *fakePressService) GetPressMediaContent(id int, mediaID int) (*PressMediaContent, error) {
	f.gotMediaEntryID = id
	f.gotMediaID = mediaID
	if f.mediaContentErr != nil {
		return nil, f.mediaContentErr
	}
	if f.mediaContentResp == nil {
		return &PressMediaContent{Content: []byte("media"), ContentType: "application/pdf", FileName: "agenda.pdf"}, nil
	}
	return f.mediaContentResp, nil
}

func (f *fakePressService) CreatePressEntry(req SavePressEntryRequest, userID *int) (*PressMutationResponse, error) {
	f.gotCreateReq = req
	f.gotCreateUserID = userID
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp == nil {
		return &PressMutationResponse{ID: 1, Title: req.Title, Status: req.Status, Visibility: req.Visibility}, nil
	}
	return f.createResp, nil
}

func (f *fakePressService) UpdatePressEntry(id int, req SavePressEntryRequest, userID *int) (*PressMutationResponse, error) {
	f.gotUpdateID = id
	f.gotUpdateReq = req
	f.gotUpdateUserID = userID
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp == nil {
		return &PressMutationResponse{ID: id, Title: req.Title, Status: req.Status, Visibility: req.Visibility}, nil
	}
	return f.updateResp, nil
}

func (f *fakePressService) DeletePressEntry(id int) error {
	f.gotDeleteID = id
	return f.deleteErr
}

func (f *fakePressService) AddPressMedia(id int, req AddPressMediaRequest, userID *int) (*AddPressMediaResponse, error) {
	f.gotAddMediaID = id
	f.gotAddMediaReq = req
	f.gotAddMediaUserID = userID
	if f.addMediaErr != nil {
		return nil, f.addMediaErr
	}
	if f.addMediaResp == nil {
		return &AddPressMediaResponse{UploadedCount: len(req.Media)}, nil
	}
	return f.addMediaResp, nil
}

func (f *fakePressService) UpdatePressMedia(id int, mediaID int, req UpdatePressMediaRequest) (*PressMediaResponse, error) {
	f.gotUpdateMediaID = id
	f.gotUpdateMediaMedia = mediaID
	f.gotUpdateMediaReq = req
	if f.updateMediaErr != nil {
		return nil, f.updateMediaErr
	}
	if f.updateMediaResp == nil {
		return &PressMediaResponse{ID: mediaID, DisplayName: req.DisplayName, FileName: req.FileName}, nil
	}
	return f.updateMediaResp, nil
}

func (f *fakePressService) ReorderPressMedia(id int, mediaIDs []int) (*ReorderPressMediaResponse, error) {
	f.gotReorderID = id
	f.gotReorderMediaIDs = mediaIDs
	if f.reorderErr != nil {
		return nil, f.reorderErr
	}
	if f.reorderResp == nil {
		return &ReorderPressMediaResponse{UpdatedCount: len(mediaIDs)}, nil
	}
	return f.reorderResp, nil
}

func (f *fakePressService) DeletePressMedia(id int, mediaIDs []int) (*DeletePressMediaResponse, error) {
	f.gotDeleteMediaID = id
	f.gotDeleteMediaIDs = mediaIDs
	if f.deleteMediaErr != nil {
		return nil, f.deleteMediaErr
	}
	if f.deleteMediaResp == nil {
		return &DeletePressMediaResponse{DeletedCount: len(mediaIDs)}, nil
	}
	return f.deleteMediaResp, nil
}

func setupPressRouter(service PressServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service)
	return r
}

func setupProtectedPressRouter(service PressServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestListAndGetPressEndpoints(t *testing.T) {
	service := &fakePressService{
		listResp: &PressListResponse{
			Items: []PressSummaryItem{{ID: 9, Title: "Spring Fair", Status: "published", Visibility: "public"}},
			Total: 1, Page: 2, PageSize: 5, TotalPages: 1,
		},
		detailResp: &PressDetailResponse{ID: 9, Title: "Spring Fair", Status: "published", Visibility: "public"},
	}
	router := setupPressRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/press?page=2&page_size=5&search=spring&status=published&visibility=public&sort_by=title&sort_order=asc", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.Page != 2 || service.gotListFilter.PageSize != 5 {
		t.Fatalf("unexpected pagination filter: %#v", service.gotListFilter)
	}
	if service.gotListFilter.SearchTerm != "spring" || service.gotListFilter.Status != "published" || service.gotListFilter.Visibility != "public" {
		t.Fatalf("unexpected list filter: %#v", service.gotListFilter)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/press/9", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDetailID != 9 {
		t.Fatalf("expected detail id 9, got %d", service.gotDetailID)
	}
}

func TestListPressEntriesEndpointDefaults(t *testing.T) {
	service := &fakePressService{}
	router := setupPressRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/press?page=bad&page_size=400", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if service.gotListFilter.Page != 1 || service.gotListFilter.PageSize != 20 {
		t.Fatalf("expected default pagination, got %#v", service.gotListFilter)
	}
	if service.gotListFilter.SortBy != "release_date" || service.gotListFilter.SortOrder != "desc" {
		t.Fatalf("expected default sort values, got %#v", service.gotListFilter)
	}
}

func TestGetPressMediaContentEndpoint(t *testing.T) {
	service := &fakePressService{
		mediaContentResp: &PressMediaContent{
			Content:     []byte("hello"),
			ContentType: "image/png",
			FileName:    "banner.png",
		},
	}
	router := setupPressRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/press/5/media/8/content", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if res.Body.String() != "hello" {
		t.Fatalf("unexpected body: %q", res.Body.String())
	}
	if service.gotMediaEntryID != 5 || service.gotMediaID != 8 {
		t.Fatalf("unexpected media ids: entry=%d media=%d", service.gotMediaEntryID, service.gotMediaID)
	}
	if got := res.Header().Get("Content-Disposition"); !strings.Contains(got, "banner.png") {
		t.Fatalf("expected content disposition filename, got %q", got)
	}
}

func TestGetPressMediaContentEndpointDefaultsAndErrors(t *testing.T) {
	service := &fakePressService{
		mediaContentResp: &PressMediaContent{
			Content:     []byte("file"),
			ContentType: "",
			FileName:    `report/"unsafe".pdf`,
		},
	}
	router := setupPressRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/press/5/media/8/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.Contains(got, "application/octet-stream") {
		t.Fatalf("expected default octet-stream content type, got %q", got)
	}
	if got := res.Header().Get("Content-Disposition"); strings.Contains(got, "/") || strings.Contains(got, `"unsafe"`) {
		t.Fatalf("expected sanitized content disposition filename, got %q", got)
	}

	router = setupPressRouter(&fakePressService{mediaContentErr: ErrPressMediaNotFound})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/press/5/media/8/content", nil)
	router.ServeHTTP(res, req)
	assertPressAPIError(t, res, http.StatusNotFound, "not_found", "press media not found")
}

func TestCreateAndUpdatePressEntryEndpoints(t *testing.T) {
	service := &fakePressService{}
	router := setupProtectedPressRouter(service)

	createBody := `{"title":"Spring Fair","release_date":"2026-05-01","status":"draft","visibility":"private","content_html":"<p>Hello</p>"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.Title != "Spring Fair" {
		t.Fatalf("unexpected create request: %#v", service.gotCreateReq)
	}
	if service.gotCreateUserID == nil || *service.gotCreateUserID != 7 {
		t.Fatalf("expected auth user id 7, got %#v", service.gotCreateUserID)
	}

	req = newPressMultipartRequest(t, http.MethodPut, "/api/press/9", `{"title":"Updated Fair","release_date":"2026-05-02","status":"published","visibility":"public","cover_image":{"display_name":"Poster"}}`, []multipartUploadTestFile{
		{Field: "cover_image_file", Filename: "poster.png", ContentType: "image/png", Data: []byte("poster")},
	})
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 9 {
		t.Fatalf("expected update id 9, got %d", service.gotUpdateID)
	}
	if service.gotUpdateReq.CoverImage == nil || string(service.gotUpdateReq.CoverImage.Content) != "poster" {
		t.Fatalf("expected uploaded cover image bytes, got %#v", service.gotUpdateReq.CoverImage)
	}
	if service.gotUpdateUserID == nil || *service.gotUpdateUserID != 7 {
		t.Fatalf("expected update auth user id 7, got %#v", service.gotUpdateUserID)
	}
}

func TestCreatePressEntryEndpointRejectsEmbeddedDataURL(t *testing.T) {
	router := setupProtectedPressRouter(&fakePressService{})

	body := `{"title":"Spring Fair","release_date":"2026-05-01","status":"draft","visibility":"private","cover_image":{"file_url":"data:image/png;base64,aGVsbG8="}}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertPressAPIError(t, res, http.StatusBadRequest, "validation_error", "use multipart/form-data with a payload field for file uploads")
}

func TestAddUpdateReorderDeletePressMediaEndpoints(t *testing.T) {
	service := &fakePressService{}
	router := setupProtectedPressRouter(service)

	req := newPressMultipartRequest(t, http.MethodPost, "/api/press/4/media", `{"media":[{"display_name":"Agenda"},{"display_name":"Minutes"}]}`, []multipartUploadTestFile{
		{Field: "media[0].file", Filename: "agenda.pdf", ContentType: "application/pdf", Data: []byte("agenda")},
		{Field: "files[]", Filename: "minutes.pdf", ContentType: "application/pdf", Data: []byte("minutes")},
	})
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotAddMediaID != 4 || len(service.gotAddMediaReq.Media) != 2 {
		t.Fatalf("unexpected add media request: id=%d req=%#v", service.gotAddMediaID, service.gotAddMediaReq)
	}
	if string(service.gotAddMediaReq.Media[0].Content) != "agenda" || string(service.gotAddMediaReq.Media[1].Content) != "minutes" {
		t.Fatalf("expected uploaded media bytes, got %#v", service.gotAddMediaReq.Media)
	}
	if service.gotAddMediaUserID == nil || *service.gotAddMediaUserID != 7 {
		t.Fatalf("expected add media auth user id 7, got %#v", service.gotAddMediaUserID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/press/4/media/10", strings.NewReader(`{"display_name":"Agenda v2","file_name":"agenda-v2.pdf"}`))
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotUpdateMediaID != 4 || service.gotUpdateMediaMedia != 10 {
		t.Fatalf("unexpected update media result: status=%d entry=%d media=%d", res.Code, service.gotUpdateMediaID, service.gotUpdateMediaMedia)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/press/4/media/order", strings.NewReader(`{"media_ids":[10,11]}`))
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotReorderID != 4 || len(service.gotReorderMediaIDs) != 2 {
		t.Fatalf("unexpected reorder result: status=%d id=%d ids=%#v", res.Code, service.gotReorderID, service.gotReorderMediaIDs)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/press/4/media", strings.NewReader(`{"media_ids":[10,11]}`))
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotDeleteMediaID != 4 || len(service.gotDeleteMediaIDs) != 2 {
		t.Fatalf("unexpected delete media result: status=%d id=%d ids=%#v", res.Code, service.gotDeleteMediaID, service.gotDeleteMediaIDs)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/press/4", nil)
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotDeleteID != 4 {
		t.Fatalf("unexpected delete entry result: status=%d id=%d", res.Code, service.gotDeleteID)
	}
}

func TestPressControllerErrorsAndProtection(t *testing.T) {
	router := setupPressRouter(&fakePressService{listErr: ErrStoreUnavailable})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/press", nil)
	router.ServeHTTP(res, req)
	assertPressAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Press service is temporarily unavailable")

	router = setupPressRouter(&fakePressService{detailErr: ErrPressEntryNotFound})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/press/99", nil)
	router.ServeHTTP(res, req)
	assertPressAPIError(t, res, http.StatusNotFound, "not_found", "press entry not found")

	router = setupProtectedPressRouter(&fakePressService{updateErr: errors.New(`ERROR: duplicate key value violates unique constraint "press_entries_title_key" (SQLSTATE 23505)`)})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/press/9", strings.NewReader(`{"title":"Spring Fair","release_date":"2026-05-01","status":"draft","visibility":"private"}`))
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertPressAPIError(t, res, http.StatusConflict, "conflict", "Unable to save press entry because a conflicting record already exists")

	router = setupProtectedPressRouter(&fakePressService{updateMediaErr: errors.New("display_name or file_name is required")})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/press/9/media/1", strings.NewReader(`{"display_name":"","file_name":""}`))
	req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertPressAPIError(t, res, http.StatusBadRequest, "validation_error", "display_name or file_name is required")

	router = setupPressRouter(&fakePressService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/press/bad", nil)
	router.ServeHTTP(res, req)
	payload := assertPressAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
		t.Fatalf("expected id validation detail, got %#v", payload)
	}

	protected := setupProtectedPressRouter(&fakePressService{})
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/press", body: `{"title":"Spring Fair"}`},
		{method: http.MethodPut, path: "/api/press/9", body: `{"title":"Spring Fair"}`},
		{method: http.MethodDelete, path: "/api/press/9"},
		{method: http.MethodPost, path: "/api/press/9/media", body: `{"media":[]}`},
		{method: http.MethodPatch, path: "/api/press/9/media/2", body: `{"display_name":"A"}`},
		{method: http.MethodPut, path: "/api/press/9/media/order", body: `{"media_ids":[1]}`},
		{method: http.MethodDelete, path: "/api/press/9/media", body: `{"media_ids":[1]}`},
	}

	for _, tc := range tests {
		res = httptest.NewRecorder()
		req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		protected.ServeHTTP(res, req)
		assertPressAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")
	}
}

func TestPressControllerPathAndBindingErrorBranches(t *testing.T) {
	router := setupProtectedPressRouter(&fakePressService{})

	tests := []struct {
		method      string
		path        string
		body        string
		wantCode    string
		wantMessage string
	}{
		{method: http.MethodPost, path: "/api/press", body: `{"title":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
		{method: http.MethodPut, path: "/api/press/bad", body: `{"title":"Spring Fair"}`, wantCode: "validation_error", wantMessage: "Invalid path parameter"},
		{method: http.MethodPost, path: "/api/press/4/media", body: `{"media":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
		{method: http.MethodPatch, path: "/api/press/4/media/bad", body: `{"display_name":"Agenda"}`, wantCode: "validation_error", wantMessage: "Invalid path parameter"},
		{method: http.MethodPut, path: "/api/press/4/media/order", body: `{"media_ids":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
		{method: http.MethodDelete, path: "/api/press/4/media", body: `{"media_ids":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
	}

	for _, tc := range tests {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertPressAPIError(t, res, http.StatusBadRequest, tc.wantCode, tc.wantMessage)
	}
}

func TestPressMutationControllerServiceErrors(t *testing.T) {
	t.Run("delete entry not found", func(t *testing.T) {
		router := setupProtectedPressRouter(&fakePressService{deleteErr: ErrPressEntryNotFound})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/press/4", nil)
		req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
		router.ServeHTTP(res, req)
		assertPressAPIError(t, res, http.StatusNotFound, "not_found", "press entry not found")
	})

	t.Run("add media validation", func(t *testing.T) {
		router := setupProtectedPressRouter(&fakePressService{addMediaErr: errors.New("media is required")})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/press/4/media", strings.NewReader(`{"media":[]}`))
		req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertPressAPIError(t, res, http.StatusBadRequest, "validation_error", "media is required")
	})

	t.Run("reorder media validation", func(t *testing.T) {
		router := setupProtectedPressRouter(&fakePressService{reorderErr: errors.New("media_ids must include every press media item exactly once")})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/press/4/media/order", strings.NewReader(`{"media_ids":[1,2]}`))
		req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertPressAPIError(t, res, http.StatusBadRequest, "validation_error", "media_ids must include every press media item exactly once")
	})

	t.Run("delete media not found", func(t *testing.T) {
		router := setupProtectedPressRouter(&fakePressService{deleteMediaErr: ErrPressMediaNotFound})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/press/4/media", strings.NewReader(`{"media_ids":[1]}`))
		req.Header.Set("Authorization", "Bearer "+signPressToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertPressAPIError(t, res, http.StatusNotFound, "not_found", "press media not found")
	})
}

func TestPressHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("auth_user_id", int32(9))

	userID := authUserID(c)
	if userID == nil || *userID != 9 {
		t.Fatalf("expected auth_user_id 9, got %#v", userID)
	}

	c = nil
	if authUserID(c) != nil {
		t.Fatal("expected nil auth user id for nil context")
	}

	if got := sanitizeContentDispositionFilename(` a/"b\c` + "\r\n;"); got != "abc" {
		t.Fatalf("unexpected sanitized filename: %q", got)
	}
	if got := pressMediaFileField(2); got != "media[2].file" {
		t.Fatalf("unexpected media field: %q", got)
	}
	for _, tc := range []struct {
		key   string
		value any
		want  int
	}{
		{key: "userID", value: int64(10), want: 10},
		{key: "user_id", value: uint(11), want: 11},
		{key: "userId", value: float64(12), want: 12},
		{key: "userId", value: "13", want: 13},
	} {
		rec = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(rec)
		c.Set(tc.key, tc.value)
		got := authUserID(c)
		if got == nil || *got != tc.want {
			t.Fatalf("expected auth user id %d from %s, got %#v", tc.want, tc.key, got)
		}
	}
	if !pressEntryUsesEmbeddedBase64(SavePressEntryRequest{CoverImage: &PressUploadInput{FileURL: " data:image/png;base64,aGVsbG8="}}) {
		t.Fatal("expected embedded cover image data url to be detected")
	}
	if !addPressMediaRequestUsesEmbeddedBase64(AddPressMediaRequest{Media: []PressUploadInput{{FileURL: "data:application/pdf;base64,aGVsbG8="}}}) {
		t.Fatal("expected embedded media data url to be detected")
	}
	if got := detectUploadedContentType("", []byte("hello")); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("expected detected text/plain content type, got %q", got)
	}

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: ""}}
	if _, ok := pathInt(c, "id"); ok {
		t.Fatal("expected empty path param to fail")
	}
	assertPressAPIError(t, rec, http.StatusBadRequest, "validation_error", "Invalid path parameter")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writePressError(c, nil)
	assertPressAPIError(t, rec, http.StatusInternalServerError, "internal_error", "Internal server error")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writePressError(c, errors.New("media_ids must include every press media item exactly once"))
	assertPressAPIError(t, rec, http.StatusBadRequest, "validation_error", "media_ids must include every press media item exactly once")

	if !isClientSafePressError(errors.New("media_ids must contain positive integers")) {
		t.Fatal("expected client-safe press validation error")
	}
	if isClientSafePressError(errors.New("database unavailable")) {
		t.Fatal("expected database error to be non-client-safe")
	}
}

func TestPressControllerNilServiceBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := &PressController{}

	tests := []struct {
		name string
		run  func(*gin.Context)
	}{
		{name: "list", run: controller.ListPressEntries},
		{name: "get", run: controller.GetPressEntry},
		{name: "media content", run: controller.GetPressMediaContent},
		{name: "create", run: controller.CreatePressEntry},
		{name: "update", run: controller.UpdatePressEntry},
		{name: "delete", run: controller.DeletePressEntry},
		{name: "add media", run: controller.AddPressMedia},
		{name: "update media", run: controller.UpdatePressMedia},
		{name: "reorder media", run: controller.ReorderPressMedia},
		{name: "delete media", run: controller.DeletePressMedia},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader(`{}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "mediaId", Value: "2"}}

			tc.run(c)
			assertPressAPIError(t, rec, http.StatusInternalServerError, "internal_error", "Internal server error")
		})
	}
}

func assertPressAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

func signPressToken(t *testing.T, secret string, userID int) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   "tester@example.com",
		"role":    "admin",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return tokenString
}

type multipartUploadTestFile struct {
	Field       string
	Filename    string
	ContentType string
	Data        []byte
}

func newPressMultipartRequest(t *testing.T, method string, target string, payload string, files []multipartUploadTestFile) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("payload", payload); err != nil {
		t.Fatalf("WriteField payload: %v", err)
	}
	for _, file := range files {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="`+file.Field+`"; filename="`+file.Filename+`"`)
		if strings.TrimSpace(file.ContentType) != "" {
			header.Set("Content-Type", file.ContentType)
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("CreatePart(%s): %v", file.Field, err)
		}
		if _, err := part.Write(file.Data); err != nil {
			t.Fatalf("part.Write(%s): %v", file.Field, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

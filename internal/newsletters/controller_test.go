package newsletters

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

type fakeNewsletterService struct {
	listResp            *NewsletterListResponse
	detailResp          *NewsletterDetailResponse
	mediaContentResp    *NewsletterMediaContent
	createResp          *NewsletterMutationResponse
	updateResp          *NewsletterMutationResponse
	addMediaResp        *AddNewsletterMediaResponse
	updateMediaResp     *NewsletterMediaResponse
	reorderResp         *ReorderNewsletterMediaResponse
	deleteMediaResp     *DeleteNewsletterMediaResponse
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
	gotListFilter       ListNewsletterFilter
	gotDetailID         int
	gotMediaEntryID     int
	gotMediaID          int
	gotCreateReq        SaveNewsletterEntryRequest
	gotCreateUserID     *int
	gotUpdateID         int
	gotUpdateReq        SaveNewsletterEntryRequest
	gotUpdateUserID     *int
	gotDeleteID         int
	gotAddMediaID       int
	gotAddMediaReq      AddNewsletterMediaRequest
	gotAddMediaUserID   *int
	gotUpdateMediaID    int
	gotUpdateMediaMedia int
	gotUpdateMediaReq   UpdateNewsletterMediaRequest
	gotReorderID        int
	gotReorderMediaIDs  []int
	gotDeleteMediaID    int
	gotDeleteMediaIDs   []int
}

func (f *fakeNewsletterService) ListNewsletterEntries(filter ListNewsletterFilter) (*NewsletterListResponse, error) {
	f.gotListFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp == nil {
		return &NewsletterListResponse{}, nil
	}
	return f.listResp, nil
}

func (f *fakeNewsletterService) GetNewsletterEntry(id int) (*NewsletterDetailResponse, error) {
	f.gotDetailID = id
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if f.detailResp == nil {
		return &NewsletterDetailResponse{ID: id, Title: "Spring Update"}, nil
	}
	return f.detailResp, nil
}

func (f *fakeNewsletterService) GetNewsletterMediaContent(id int, mediaID int) (*NewsletterMediaContent, error) {
	f.gotMediaEntryID = id
	f.gotMediaID = mediaID
	if f.mediaContentErr != nil {
		return nil, f.mediaContentErr
	}
	if f.mediaContentResp == nil {
		return &NewsletterMediaContent{Content: []byte("media"), ContentType: "application/pdf", FileName: "agenda.pdf"}, nil
	}
	return f.mediaContentResp, nil
}

func (f *fakeNewsletterService) CreateNewsletterEntry(req SaveNewsletterEntryRequest, userID *int) (*NewsletterMutationResponse, error) {
	f.gotCreateReq = req
	f.gotCreateUserID = userID
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp == nil {
		return &NewsletterMutationResponse{ID: 1, Title: req.Title, Category: req.Category, Status: req.Status, Visibility: req.Visibility}, nil
	}
	return f.createResp, nil
}

func (f *fakeNewsletterService) UpdateNewsletterEntry(id int, req SaveNewsletterEntryRequest, userID *int) (*NewsletterMutationResponse, error) {
	f.gotUpdateID = id
	f.gotUpdateReq = req
	f.gotUpdateUserID = userID
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp == nil {
		return &NewsletterMutationResponse{ID: id, Title: req.Title, Category: req.Category, Status: req.Status, Visibility: req.Visibility}, nil
	}
	return f.updateResp, nil
}

func (f *fakeNewsletterService) DeleteNewsletterEntry(id int) error {
	f.gotDeleteID = id
	return f.deleteErr
}

func (f *fakeNewsletterService) AddNewsletterMedia(id int, req AddNewsletterMediaRequest, userID *int) (*AddNewsletterMediaResponse, error) {
	f.gotAddMediaID = id
	f.gotAddMediaReq = req
	f.gotAddMediaUserID = userID
	if f.addMediaErr != nil {
		return nil, f.addMediaErr
	}
	if f.addMediaResp == nil {
		return &AddNewsletterMediaResponse{UploadedCount: len(req.Media)}, nil
	}
	return f.addMediaResp, nil
}

func (f *fakeNewsletterService) UpdateNewsletterMedia(id int, mediaID int, req UpdateNewsletterMediaRequest) (*NewsletterMediaResponse, error) {
	f.gotUpdateMediaID = id
	f.gotUpdateMediaMedia = mediaID
	f.gotUpdateMediaReq = req
	if f.updateMediaErr != nil {
		return nil, f.updateMediaErr
	}
	if f.updateMediaResp == nil {
		return &NewsletterMediaResponse{ID: mediaID, DisplayName: req.DisplayName, FileName: req.FileName}, nil
	}
	return f.updateMediaResp, nil
}

func (f *fakeNewsletterService) ReorderNewsletterMedia(id int, mediaIDs []int) (*ReorderNewsletterMediaResponse, error) {
	f.gotReorderID = id
	f.gotReorderMediaIDs = mediaIDs
	if f.reorderErr != nil {
		return nil, f.reorderErr
	}
	if f.reorderResp == nil {
		return &ReorderNewsletterMediaResponse{UpdatedCount: len(mediaIDs)}, nil
	}
	return f.reorderResp, nil
}

func (f *fakeNewsletterService) DeleteNewsletterMedia(id int, mediaIDs []int) (*DeleteNewsletterMediaResponse, error) {
	f.gotDeleteMediaID = id
	f.gotDeleteMediaIDs = mediaIDs
	if f.deleteMediaErr != nil {
		return nil, f.deleteMediaErr
	}
	if f.deleteMediaResp == nil {
		return &DeleteNewsletterMediaResponse{DeletedCount: len(mediaIDs)}, nil
	}
	return f.deleteMediaResp, nil
}

func setupNewsletterRouter(service NewsletterServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service)
	return r
}

func setupProtectedNewsletterRouter(service NewsletterServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestListAndGetNewsletterEndpoints(t *testing.T) {
	service := &fakeNewsletterService{
		listResp: &NewsletterListResponse{
			Items: []NewsletterSummaryItem{{ID: 9, Title: "Spring Update", Category: "csaa", Status: "published", Visibility: "public"}},
			Total: 1, Page: 2, PageSize: 5, TotalPages: 1,
		},
		detailResp: &NewsletterDetailResponse{ID: 9, Title: "Spring Update", Category: "csaa", Status: "published", Visibility: "public"},
	}
	router := setupNewsletterRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/newsletters?page=2&page_size=5&search=spring&status=published&visibility=public&sort_by=title&sort_order=asc", nil)
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
	req = httptest.NewRequest(http.MethodGet, "/api/newsletters/9", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDetailID != 9 {
		t.Fatalf("expected detail id 9, got %d", service.gotDetailID)
	}
}

func TestListNewsletterEntriesEndpointDefaults(t *testing.T) {
	service := &fakeNewsletterService{}
	router := setupNewsletterRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/newsletters?page=bad&page_size=400", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if service.gotListFilter.Page != 1 || service.gotListFilter.PageSize != 20 {
		t.Fatalf("expected default pagination, got %#v", service.gotListFilter)
	}
	if service.gotListFilter.SortBy != "send_date" || service.gotListFilter.SortOrder != "desc" {
		t.Fatalf("expected default sort values, got %#v", service.gotListFilter)
	}
}

func TestGetNewsletterMediaContentEndpoint(t *testing.T) {
	service := &fakeNewsletterService{
		mediaContentResp: &NewsletterMediaContent{
			Content:     []byte("hello"),
			ContentType: "image/png",
			FileName:    "banner.png",
		},
	}
	router := setupNewsletterRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/newsletters/5/media/8/content", nil)
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

func TestGetNewsletterMediaContentEndpointDefaultsAndErrors(t *testing.T) {
	service := &fakeNewsletterService{
		mediaContentResp: &NewsletterMediaContent{
			Content:     []byte("file"),
			ContentType: "",
			FileName:    `report/"unsafe".pdf`,
		},
	}
	router := setupNewsletterRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/newsletters/5/media/8/content", nil)
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

	router = setupNewsletterRouter(&fakeNewsletterService{mediaContentErr: ErrNewsletterMediaNotFound})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/newsletters/5/media/8/content", nil)
	router.ServeHTTP(res, req)
	assertNewsletterAPIError(t, res, http.StatusNotFound, "not_found", "newsletter media not found")
}

func TestCreateAndUpdateNewsletterEntryEndpoints(t *testing.T) {
	service := &fakeNewsletterService{}
	router := setupProtectedNewsletterRouter(service)

	createBody := `{"title":"Spring Update","category":"csaa","send_date":"2026-05-01","status":"draft","visibility":"private","content_html":"<p>Hello</p>"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/newsletters", strings.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.Title != "Spring Update" || service.gotCreateReq.Category != "csaa" {
		t.Fatalf("unexpected create request: %#v", service.gotCreateReq)
	}
	if service.gotCreateUserID == nil || *service.gotCreateUserID != 7 {
		t.Fatalf("expected auth user id 7, got %#v", service.gotCreateUserID)
	}

	updateBody := `{"title":"Updated Spring","category":"cst","send_date":"2026-05-02","status":"published","visibility":"public"}`
	req = httptest.NewRequest(http.MethodPut, "/api/newsletters/9", strings.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 9 {
		t.Fatalf("expected update id 9, got %d", service.gotUpdateID)
	}
	if service.gotUpdateReq.Category != "cst" || service.gotUpdateReq.Status != "published" {
		t.Fatalf("unexpected update request: %#v", service.gotUpdateReq)
	}
	if service.gotUpdateUserID == nil || *service.gotUpdateUserID != 7 {
		t.Fatalf("expected update auth user id 7, got %#v", service.gotUpdateUserID)
	}
}

func TestAddUpdateReorderDeleteNewsletterMediaEndpoints(t *testing.T) {
	service := &fakeNewsletterService{}
	router := setupProtectedNewsletterRouter(service)

	req := newNewsletterMultipartRequest(t, http.MethodPost, "/api/newsletters/4/media", `{"media":[{"display_name":"Agenda"},{"display_name":"Minutes"}]}`, []multipartUploadTestFile{
		{Field: "media[0].file", Filename: "agenda.pdf", ContentType: "application/pdf", Data: []byte("agenda")},
		{Field: "files[]", Filename: "minutes.pdf", ContentType: "application/pdf", Data: []byte("minutes")},
	})
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
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
	var addPayload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&addPayload); err != nil {
		t.Fatalf("decode add media response: %v", err)
	}
	if addPayload["uploadedCount"] != float64(2) {
		t.Fatalf("expected uploadedCount 2, got %#v", addPayload)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/newsletters/4/media/10", strings.NewReader(`{"display_name":"Agenda v2","file_name":"agenda-v2.pdf"}`))
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotUpdateMediaID != 4 || service.gotUpdateMediaMedia != 10 {
		t.Fatalf("unexpected update media result: status=%d entry=%d media=%d", res.Code, service.gotUpdateMediaID, service.gotUpdateMediaMedia)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/newsletters/4/media/order", strings.NewReader(`{"media_ids":[10,11]}`))
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotReorderID != 4 || len(service.gotReorderMediaIDs) != 2 {
		t.Fatalf("unexpected reorder result: status=%d id=%d ids=%#v", res.Code, service.gotReorderID, service.gotReorderMediaIDs)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/newsletters/4/media", strings.NewReader(`{"media_ids":[10,11]}`))
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotDeleteMediaID != 4 || len(service.gotDeleteMediaIDs) != 2 {
		t.Fatalf("unexpected delete media result: status=%d id=%d ids=%#v", res.Code, service.gotDeleteMediaID, service.gotDeleteMediaIDs)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/newsletters/4", nil)
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotDeleteID != 4 {
		t.Fatalf("unexpected delete entry result: status=%d id=%d", res.Code, service.gotDeleteID)
	}
}

func TestNewsletterControllerErrorsAndProtection(t *testing.T) {
	router := setupNewsletterRouter(&fakeNewsletterService{listErr: ErrStoreUnavailable})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/newsletters", nil)
	router.ServeHTTP(res, req)
	assertNewsletterAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Newsletter service is temporarily unavailable")

	router = setupNewsletterRouter(&fakeNewsletterService{detailErr: ErrNewsletterEntryNotFound})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/newsletters/99", nil)
	router.ServeHTTP(res, req)
	assertNewsletterAPIError(t, res, http.StatusNotFound, "not_found", "newsletter entry not found")

	router = setupProtectedNewsletterRouter(&fakeNewsletterService{updateErr: errors.New(`ERROR: duplicate key value violates unique constraint "newsletter_entries_title_key" (SQLSTATE 23505)`)})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/newsletters/9", strings.NewReader(`{"title":"Spring Update","category":"csaa","send_date":"2026-05-01","status":"draft","visibility":"private"}`))
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertNewsletterAPIError(t, res, http.StatusConflict, "conflict", "Unable to save newsletter because a conflicting record already exists")

	router = setupProtectedNewsletterRouter(&fakeNewsletterService{updateMediaErr: errors.New("display_name or file_name is required")})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/newsletters/9/media/1", strings.NewReader(`{"display_name":"","file_name":""}`))
	req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertNewsletterAPIError(t, res, http.StatusBadRequest, "validation_error", "display_name or file_name is required")

	router = setupNewsletterRouter(&fakeNewsletterService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/newsletters/bad", nil)
	router.ServeHTTP(res, req)
	payload := assertNewsletterAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
		t.Fatalf("expected id validation detail, got %#v", payload)
	}

	protected := setupProtectedNewsletterRouter(&fakeNewsletterService{})
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/newsletters", body: `{"title":"Spring Update"}`},
		{method: http.MethodPut, path: "/api/newsletters/9", body: `{"title":"Spring Update"}`},
		{method: http.MethodDelete, path: "/api/newsletters/9"},
		{method: http.MethodPost, path: "/api/newsletters/9/media", body: `{"media":[]}`},
		{method: http.MethodPatch, path: "/api/newsletters/9/media/2", body: `{"display_name":"A"}`},
		{method: http.MethodPut, path: "/api/newsletters/9/media/order", body: `{"media_ids":[1]}`},
		{method: http.MethodDelete, path: "/api/newsletters/9/media", body: `{"media_ids":[1]}`},
	}

	for _, tc := range tests {
		res = httptest.NewRecorder()
		req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		protected.ServeHTTP(res, req)
		assertNewsletterAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")
	}
}

func TestNewsletterControllerPathAndBindingErrorBranches(t *testing.T) {
	router := setupProtectedNewsletterRouter(&fakeNewsletterService{})

	tests := []struct {
		method      string
		path        string
		body        string
		wantCode    string
		wantMessage string
	}{
		{method: http.MethodPost, path: "/api/newsletters", body: `{"title":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
		{method: http.MethodPut, path: "/api/newsletters/bad", body: `{"title":"Spring Update"}`, wantCode: "validation_error", wantMessage: "Invalid path parameter"},
		{method: http.MethodPost, path: "/api/newsletters/4/media", body: `{"media":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
		{method: http.MethodPatch, path: "/api/newsletters/4/media/bad", body: `{"display_name":"Agenda"}`, wantCode: "validation_error", wantMessage: "Invalid path parameter"},
		{method: http.MethodPut, path: "/api/newsletters/4/media/order", body: `{"media_ids":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
		{method: http.MethodDelete, path: "/api/newsletters/4/media", body: `{"media_ids":`, wantCode: "invalid_json", wantMessage: "Request body contains invalid JSON"},
	}

	for _, tc := range tests {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertNewsletterAPIError(t, res, http.StatusBadRequest, tc.wantCode, tc.wantMessage)
	}
}

func TestNewsletterMutationControllerServiceErrors(t *testing.T) {
	t.Run("delete entry not found", func(t *testing.T) {
		router := setupProtectedNewsletterRouter(&fakeNewsletterService{deleteErr: ErrNewsletterEntryNotFound})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/newsletters/4", nil)
		req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
		router.ServeHTTP(res, req)
		assertNewsletterAPIError(t, res, http.StatusNotFound, "not_found", "newsletter entry not found")
	})

	t.Run("add media validation", func(t *testing.T) {
		router := setupProtectedNewsletterRouter(&fakeNewsletterService{addMediaErr: errors.New("media is required")})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/newsletters/4/media", strings.NewReader(`{"media":[]}`))
		req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertNewsletterAPIError(t, res, http.StatusBadRequest, "validation_error", "media is required")
	})

	t.Run("reorder media validation", func(t *testing.T) {
		router := setupProtectedNewsletterRouter(&fakeNewsletterService{reorderErr: errors.New("media_ids must include every newsletter media item exactly once")})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/newsletters/4/media/order", strings.NewReader(`{"media_ids":[1,2]}`))
		req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertNewsletterAPIError(t, res, http.StatusBadRequest, "validation_error", "media_ids must include every newsletter media item exactly once")
	})

	t.Run("delete media not found", func(t *testing.T) {
		router := setupProtectedNewsletterRouter(&fakeNewsletterService{deleteMediaErr: ErrNewsletterMediaNotFound})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/newsletters/4/media", strings.NewReader(`{"media_ids":[1]}`))
		req.Header.Set("Authorization", "Bearer "+signNewsletterToken(t, "test-secret", 7))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(res, req)
		assertNewsletterAPIError(t, res, http.StatusNotFound, "not_found", "newsletter media not found")
	})
}

func TestNewsletterHelpers(t *testing.T) {
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
	if got := newsletterMediaFileField(2); got != "media[2].file" {
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
	if !addNewsletterMediaRequestUsesEmbeddedBase64(AddNewsletterMediaRequest{Media: []NewsletterUploadInput{{FileURL: "data:application/pdf;base64,aGVsbG8="}}}) {
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
	assertNewsletterAPIError(t, rec, http.StatusBadRequest, "validation_error", "Invalid path parameter")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writeNewsletterError(c, nil)
	assertNewsletterAPIError(t, rec, http.StatusInternalServerError, "internal_error", "Internal server error")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writeNewsletterError(c, errors.New("media_ids must include every newsletter media item exactly once"))
	assertNewsletterAPIError(t, rec, http.StatusBadRequest, "validation_error", "media_ids must include every newsletter media item exactly once")

	if !isClientSafeNewsletterError(errors.New("media_ids must contain positive integers")) {
		t.Fatal("expected client-safe newsletter validation error")
	}
	if isClientSafeNewsletterError(errors.New("database unavailable")) {
		t.Fatal("expected database error to be non-client-safe")
	}
}

func TestNewsletterControllerNilServiceBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := &NewsletterController{}

	tests := []struct {
		name string
		run  func(*gin.Context)
	}{
		{name: "list", run: controller.ListNewsletterEntries},
		{name: "get", run: controller.GetNewsletterEntry},
		{name: "media content", run: controller.GetNewsletterMediaContent},
		{name: "create", run: controller.CreateNewsletterEntry},
		{name: "update", run: controller.UpdateNewsletterEntry},
		{name: "delete", run: controller.DeleteNewsletterEntry},
		{name: "add media", run: controller.AddNewsletterMedia},
		{name: "update media", run: controller.UpdateNewsletterMedia},
		{name: "reorder media", run: controller.ReorderNewsletterMedia},
		{name: "delete media", run: controller.DeleteNewsletterMedia},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/newsletters", strings.NewReader(`{}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "mediaId", Value: "2"}}

			tc.run(c)
			assertNewsletterAPIError(t, rec, http.StatusInternalServerError, "internal_error", "Internal server error")
		})
	}
}

func assertNewsletterAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

func signNewsletterToken(t *testing.T, secret string, userID int) string {
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

func newNewsletterMultipartRequest(t *testing.T, method string, target string, payload string, files []multipartUploadTestFile) *http.Request {
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

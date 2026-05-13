package pages

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/apiresponse"
	authpkg "nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
)

type fakePageService struct {
	listResp      *PageListResponse
	detailResp    *PageDetailResponse
	heroResp      *PageHeroImageContent
	createResp    *PageMutationResponse
	updateResp    *PageMutationResponse
	listErr       error
	detailErr     error
	heroErr       error
	createErr     error
	updateErr     error
	deleteErr     error
	gotListFilter PageListFilters
	gotGetID      int
	gotHeroID     int
	gotCreateReq  SavePageRequest
	gotUpdateID   int
	gotUpdateReq  SavePageRequest
	gotDeleteID   int
}

func (s *fakePageService) ListPages(filter PageListFilters) (*PageListResponse, error) {
	s.gotListFilter = filter
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResp == nil {
		return &PageListResponse{}, nil
	}
	return s.listResp, nil
}

func (s *fakePageService) GetPage(id int) (*PageDetailResponse, error) {
	s.gotGetID = id
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	if s.detailResp == nil {
		return &PageDetailResponse{ID: id, PageTitle: "Homepage"}, nil
	}
	return s.detailResp, nil
}

func (s *fakePageService) GetPageHeroImageContent(id int) (*PageHeroImageContent, error) {
	s.gotHeroID = id
	if s.heroErr != nil {
		return nil, s.heroErr
	}
	if s.heroResp == nil {
		return &PageHeroImageContent{Content: []byte("ok"), ContentType: "image/png", FileName: "hero.png"}, nil
	}
	return s.heroResp, nil
}

func (s *fakePageService) CreatePage(req SavePageRequest) (*PageMutationResponse, error) {
	s.gotCreateReq = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createResp == nil {
		return &PageMutationResponse{ID: 1, PageTitle: req.PageTitle, URLSlug: req.URLSlug, ParentID: req.ParentID, Status: req.Status}, nil
	}
	return s.createResp, nil
}

func (s *fakePageService) UpdatePage(id int, req SavePageRequest) (*PageMutationResponse, error) {
	s.gotUpdateID = id
	s.gotUpdateReq = req
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.updateResp == nil {
		return &PageMutationResponse{ID: id, PageTitle: req.PageTitle, URLSlug: req.URLSlug, ParentID: req.ParentID, Status: req.Status}, nil
	}
	return s.updateResp, nil
}

func (s *fakePageService) DeletePage(id int) error {
	s.gotDeleteID = id
	return s.deleteErr
}

func setupPageRouter(service PageServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service)
	return r
}

func setupProtectedPageRouter(service PageServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireAPIKey(&config.Config{APIKey: "test-api-key"}))
	return r
}

func TestListPagesEndpoint(t *testing.T) {
	service := &fakePageService{
		listResp: &PageListResponse{
			Items: []PageListItem{
				{ID: 9, PageTitle: "Homepage", URLSlug: "/home", Status: PageStatusPublished},
			},
		},
	}
	router := setupPageRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/pages?page=2&page_size=5&search=home&status=published", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.Page != 2 || service.gotListFilter.PageSize != 5 || service.gotListFilter.SearchTerm != "home" {
		t.Fatalf("unexpected list filter: %#v", service.gotListFilter)
	}
	if !service.gotListFilter.UsePagination {
		t.Fatalf("expected pagination to be enabled when page params are provided, got %#v", service.gotListFilter)
	}
}

func TestListPagesEndpointWithoutPaginationParamsReturnsAllPages(t *testing.T) {
	service := &fakePageService{
		listResp: &PageListResponse{
			Items: []PageListItem{
				{ID: 9, PageTitle: "Homepage", URLSlug: "/home", Status: PageStatusPublished},
				{ID: 10, PageTitle: "About", URLSlug: "/about", Status: PageStatusDraft},
			},
		},
	}
	router := setupPageRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/pages?sort_by=last_modified&sort_order=desc", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.UsePagination {
		t.Fatalf("expected pagination to be disabled when page params are omitted, got %#v", service.gotListFilter)
	}
	if service.gotListFilter.Page != 0 || service.gotListFilter.PageSize != 0 {
		t.Fatalf("expected omitted page params to stay unset, got %#v", service.gotListFilter)
	}
}

func TestGetPageAndHeroEndpoints(t *testing.T) {
	service := &fakePageService{
		detailResp: &PageDetailResponse{ID: 12, PageTitle: "About Us", URLSlug: "/about-us"},
		heroResp: &PageHeroImageContent{
			Content:     []byte("hero"),
			ContentType: "image/png",
			FileName:    "hero.png",
		},
	}
	router := setupPageRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/pages/12", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotGetID != 12 {
		t.Fatalf("expected get id 12, got %d", service.gotGetID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/pages/12/hero/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if res.Body.String() != "hero" {
		t.Fatalf("unexpected body: %q", res.Body.String())
	}
	if got := res.Header().Get("Content-Disposition"); !strings.Contains(got, "hero.png") {
		t.Fatalf("expected content disposition filename, got %q", got)
	}
}

func TestCreateAndUpdatePageEndpointsRequireAPIKey(t *testing.T) {
	service := &fakePageService{}
	router := setupProtectedPageRouter(service)

	createBody := `{"page_title":"Homepage","url_slug":"/home","parent_id":3,"status":"draft","hero_image_enabled":false}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(createBody))
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.CreatedBy != nil || service.gotCreateReq.ModifiedBy != nil {
		t.Fatalf("expected API key writes to omit auth user ids, got %#v", service.gotCreateReq)
	}
	if service.gotCreateReq.ParentID == nil || *service.gotCreateReq.ParentID != 3 {
		t.Fatalf("expected create request parent_id=3, got %#v", service.gotCreateReq)
	}

	updateBody := `{"page_title":"Homepage","url_slug":"/home/updated","parent_id":4,"status":"published","hero_image_enabled":true}`
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/pages/12", strings.NewReader(updateBody))
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 12 {
		t.Fatalf("expected update id 12, got %d", service.gotUpdateID)
	}
	if service.gotUpdateReq.ModifiedBy != nil {
		t.Fatalf("expected API key writes to omit modified_by, got %#v", service.gotUpdateReq)
	}
	if service.gotUpdateReq.ParentID == nil || *service.gotUpdateReq.ParentID != 4 {
		t.Fatalf("expected update request parent_id=4, got %#v", service.gotUpdateReq)
	}
}

func TestCreatePageEndpointAcceptsMultipartHeroUpload(t *testing.T) {
	service := &fakePageService{}
	router := setupProtectedPageRouter(service)

	req := newPageMultipartRequest(t, http.MethodPost, "/api/pages", `{"page_title":"Homepage","url_slug":"/home","status":"draft","hero_image_enabled":true}`, map[string]multipartUploadTestFile{
		"hero_image_file": {Filename: "hero.png", Data: []byte("hello")},
	})
	req.Header.Set("X-API-Key", "test-api-key")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.HeroImage == nil {
		t.Fatalf("expected hero image to be forwarded, got %#v", service.gotCreateReq)
	}
	if string(service.gotCreateReq.HeroImage.Content) != "hello" {
		t.Fatalf("expected multipart file bytes to be forwarded, got %#v", service.gotCreateReq.HeroImage)
	}
	if service.gotCreateReq.HeroImage.FileName != "hero.png" {
		t.Fatalf("expected uploaded filename hero.png, got %#v", service.gotCreateReq.HeroImage)
	}
}

func TestCreatePageEndpointRejectsBase64UploadPayload(t *testing.T) {
	service := &fakePageService{}
	router := setupProtectedPageRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(`{"page_title":"Homepage","url_slug":"/home","status":"draft","hero_image_enabled":true,"hero_image":{"file_name":"hero.png","mime_type":"image/png","data_base64":"aGVsbG8="}}`))
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertPageAPIError(t, res, http.StatusBadRequest, "validation_error", "use multipart/form-data with a payload field for file uploads")
}

func TestDeletePageEndpoint(t *testing.T) {
	service := &fakePageService{}
	router := setupProtectedPageRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/pages/44", nil)
	req.Header.Set("X-API-Key", "test-api-key")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDeleteID != 44 {
		t.Fatalf("expected delete id 44, got %d", service.gotDeleteID)
	}
}

func TestPageEndpointErrorsAndProtection(t *testing.T) {
	router := setupPageRouter(&fakePageService{detailErr: ErrPageNotFound})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/pages/99", nil)
	router.ServeHTTP(res, req)
	assertPageAPIError(t, res, http.StatusNotFound, "not_found", "page not found")

	router = setupPageRouter(&fakePageService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/pages/bad", nil)
	router.ServeHTTP(res, req)
	payload := assertPageAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
		t.Fatalf("expected id validation detail, got %#v", payload)
	}

	protected := setupProtectedPageRouter(&fakePageService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(`{"page_title":"Homepage"}`))
	req.Header.Set("Content-Type", "application/json")
	protected.ServeHTTP(res, req)
	assertPageAPIError(t, res, http.StatusUnauthorized, "missing_api_key", "Missing API key")

	protected = setupProtectedPageRouter(&fakePageService{updateErr: ErrPageModuleManaged})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/pages/12", strings.NewReader(`{"page_title":"Events","url_slug":"/events","status":"published","hero_image_enabled":false}`))
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("Content-Type", "application/json")
	protected.ServeHTTP(res, req)
	assertPageAPIError(t, res, http.StatusBadRequest, "validation_error", "module pages are managed elsewhere in the CMS and cannot be edited here")

	protected = setupProtectedPageRouter(&fakePageService{deleteErr: ErrPageModuleManaged})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/pages/12", nil)
	req.Header.Set("X-API-Key", "test-api-key")
	protected.ServeHTTP(res, req)
	assertPageAPIError(t, res, http.StatusBadRequest, "validation_error", "module pages are managed elsewhere in the CMS and cannot be edited here")
}

func TestWritePageErrorAndHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writePageError(c, ErrMediaBucketNotConfigured)
	assertPageAPIError(t, rec, http.StatusServiceUnavailable, "service_unavailable", "Page service is temporarily unavailable")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writePageError(c, errors.New("page_title is required"))
	assertPageAPIError(t, rec, http.StatusBadRequest, "validation_error", "page_title is required")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writePageError(c, errors.New(`url_slug must be prefixed with parent page slug "/about"`))
	assertPageAPIError(t, rec, http.StatusBadRequest, "validation_error", `url_slug must be prefixed with parent page slug "/about"`)

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writePageError(c, errors.New(`ERROR: duplicate key value violates unique constraint "pages_url_slug_key" (SQLSTATE 23505)`))
	assertPageAPIError(t, rec, http.StatusConflict, "conflict", "Unable to save page because a conflicting record already exists")

	if got := parseQueryInt("bad"); got != -1 {
		t.Fatalf("expected parseQueryInt invalid marker -1, got %d", got)
	}
	if got := sanitizeContentDispositionFilename(" hero.png\r\n "); got != "hero.png" {
		t.Fatalf("unexpected sanitized filename: %q", got)
	}

	c.Set("auth_user_id", int64(9))
	authUserID := authUserIDFromContext(c)
	if authUserID == nil || *authUserID != 9 {
		t.Fatalf("expected auth user id 9, got %#v", authUserID)
	}
}

func assertPageAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

type multipartUploadTestFile struct {
	Filename string
	Data     []byte
}

func newPageMultipartRequest(t *testing.T, method string, target string, payload string, files map[string]multipartUploadTestFile) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("payload", payload); err != nil {
		t.Fatalf("WriteField payload: %v", err)
	}
	for field, file := range files {
		part, err := writer.CreateFormFile(field, file.Filename)
		if err != nil {
			t.Fatalf("CreateFormFile(%s): %v", field, err)
		}
		if _, err := part.Write(file.Data); err != nil {
			t.Fatalf("part.Write(%s): %v", field, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

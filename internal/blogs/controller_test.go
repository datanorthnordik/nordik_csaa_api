package blogs

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
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

type fakeBlogService struct {
	listResp            *BlogListResponse
	detailResp          *BlogDetailResponse
	coverResp           *BlogMediaContent
	sectionImageResp    *BlogMediaContent
	animationImageResp  *BlogMediaContent
	createResp          *BlogMutationResponse
	updateResp          *BlogMutationResponse
	listErr             error
	detailErr           error
	coverErr            error
	sectionImageErr     error
	animationImageErr   error
	createErr           error
	updateErr           error
	deleteErr           error
	gotListFilter       BlogListFilters
	gotGetID            int
	gotCoverID          int
	gotSectionImageID   int
	gotSectionID        int
	gotAnimationID      int
	gotAnimationSection int
	gotAnimationItem    int
	gotCreateReq        SaveBlogRequest
	gotUpdateID         int
	gotUpdateReq        SaveBlogRequest
	gotDeleteID         int
}

func (s *fakeBlogService) ListBlogs(filter BlogListFilters) (*BlogListResponse, error) {
	s.gotListFilter = filter
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResp == nil {
		return &BlogListResponse{}, nil
	}
	return s.listResp, nil
}

func (s *fakeBlogService) GetBlog(id int) (*BlogDetailResponse, error) {
	s.gotGetID = id
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	if s.detailResp == nil {
		return &BlogDetailResponse{ID: id, Heading: "Story"}, nil
	}
	return s.detailResp, nil
}

func (s *fakeBlogService) GetBlogCoverImageContent(id int) (*BlogMediaContent, error) {
	s.gotCoverID = id
	if s.coverErr != nil {
		return nil, s.coverErr
	}
	if s.coverResp == nil {
		return &BlogMediaContent{
			Content:     []byte("cover"),
			ContentType: "image/png",
			FileName:    "cover.png",
		}, nil
	}
	return s.coverResp, nil
}

func (s *fakeBlogService) GetBlogSectionImageContent(id int, sectionID int) (*BlogMediaContent, error) {
	s.gotSectionImageID = id
	s.gotSectionID = sectionID
	if s.sectionImageErr != nil {
		return nil, s.sectionImageErr
	}
	if s.sectionImageResp == nil {
		return &BlogMediaContent{
			Content:     []byte("section-image"),
			ContentType: "image/png",
			FileName:    "section.png",
		}, nil
	}
	return s.sectionImageResp, nil
}

func (s *fakeBlogService) GetBlogAnimationItemImageContent(id int, sectionID int, itemID int) (*BlogMediaContent, error) {
	s.gotAnimationID = id
	s.gotAnimationSection = sectionID
	s.gotAnimationItem = itemID
	if s.animationImageErr != nil {
		return nil, s.animationImageErr
	}
	if s.animationImageResp == nil {
		return &BlogMediaContent{
			Content:     []byte("animation-image"),
			ContentType: "image/png",
			FileName:    "animation.png",
		}, nil
	}
	return s.animationImageResp, nil
}

func (s *fakeBlogService) CreateBlog(req SaveBlogRequest) (*BlogMutationResponse, error) {
	s.gotCreateReq = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createResp == nil {
		return &BlogMutationResponse{
			ID:          1,
			PublishDate: time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC),
			Heading:     req.Heading,
		}, nil
	}
	return s.createResp, nil
}

func (s *fakeBlogService) UpdateBlog(id int, req SaveBlogRequest) (*BlogMutationResponse, error) {
	s.gotUpdateID = id
	s.gotUpdateReq = req
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.updateResp == nil {
		return &BlogMutationResponse{
			ID:          id,
			PublishDate: time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC),
			Heading:     req.Heading,
		}, nil
	}
	return s.updateResp, nil
}

func (s *fakeBlogService) DeleteBlog(id int) error {
	s.gotDeleteID = id
	return s.deleteErr
}

func setupBlogRouter(service BlogServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service)
	return r
}

func setupProtectedBlogRouter(service BlogServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestListBlogsEndpoint(t *testing.T) {
	service := &fakeBlogService{
		listResp: &BlogListResponse{
			Items: []BlogListItem{{ID: 9, Heading: "Shirley"}},
		},
	}
	router := setupBlogRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/blogs?page=2&page_size=5&search=shirley", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.Page != 2 || service.gotListFilter.PageSize != 5 || service.gotListFilter.SearchTerm != "shirley" {
		t.Fatalf("unexpected list filter: %#v", service.gotListFilter)
	}
	if !service.gotListFilter.UsePagination {
		t.Fatalf("expected pagination to be enabled, got %#v", service.gotListFilter)
	}
}

func TestGetBlogAndAssetEndpoints(t *testing.T) {
	service := &fakeBlogService{
		detailResp: &BlogDetailResponse{ID: 12, Heading: "Tree Planting"},
	}
	router := setupBlogRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/blogs/12", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotGetID != 12 {
		t.Fatalf("expected get id 12, got %d", service.gotGetID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/blogs/12/cover/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if res.Body.String() != "cover" {
		t.Fatalf("unexpected cover body: %q", res.Body.String())
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/blogs/12/sections/31/image/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotSectionImageID != 12 || service.gotSectionID != 31 {
		t.Fatalf("unexpected section image params: %d %d", service.gotSectionImageID, service.gotSectionID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/blogs/12/sections/42/items/73/image/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotAnimationID != 12 || service.gotAnimationSection != 42 || service.gotAnimationItem != 73 {
		t.Fatalf(
			"unexpected animation image params: %d %d %d",
			service.gotAnimationID,
			service.gotAnimationSection,
			service.gotAnimationItem,
		)
	}
}

func TestCreateAndUpdateBlogEndpointsRequireBearerAuth(t *testing.T) {
	service := &fakeBlogService{}
	router := setupProtectedBlogRouter(service)

	createBody := `{"publish_date":"2026-06-19","heading":"Shirley","description":"Story summary","cover_image":{"file_url":"gs://bucket/blogs/1/cover.png","gcp_object_key":"blogs/1/cover.png"}}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/blogs", strings.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+signedBlogTestToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.CreatedBy == nil || *service.gotCreateReq.CreatedBy != 7 {
		t.Fatalf("expected created_by to be forwarded, got %#v", service.gotCreateReq)
	}

	updateBody := `{"publish_date":"2026-06-20","heading":"Updated","description":"Updated summary","cover_image":{"file_url":"gs://bucket/blogs/1/cover.png","gcp_object_key":"blogs/1/cover.png"}}`
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/blogs/12", strings.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+signedBlogTestToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 12 {
		t.Fatalf("expected update id 12, got %d", service.gotUpdateID)
	}
	if service.gotUpdateReq.UpdatedBy == nil || *service.gotUpdateReq.UpdatedBy != 7 {
		t.Fatalf("expected updated_by to be forwarded, got %#v", service.gotUpdateReq)
	}
}

func TestCreateBlogEndpointAcceptsMultipartUploads(t *testing.T) {
	service := &fakeBlogService{}
	router := setupProtectedBlogRouter(service)

	req := newBlogMultipartRequest(
		t,
		http.MethodPost,
		"/api/blogs",
		`{"publish_date":"2026-06-19","heading":"Shirley","description":"Story summary","blog_detail":{"sections":[{"section_name":"Image","section_type":"image","sort_order":0,"is_enabled":true,"settings":{},"image":{"caption":"Shirley"}}]}}`,
		map[string]multipartUploadTestFile{
			"cover_image_file":                         {Filename: "cover.png", Data: []byte("cover-bytes")},
			"blog_detail.sections[0].image.asset.file": {Filename: "story.png", Data: []byte("story-bytes")},
		},
	)
	req.Header.Set("Authorization", "Bearer "+signedBlogTestToken(t, "test-secret", 7))

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.CoverImage == nil || string(service.gotCreateReq.CoverImage.Content) != "cover-bytes" {
		t.Fatalf("expected cover image bytes to be forwarded, got %#v", service.gotCreateReq.CoverImage)
	}
	if service.gotCreateReq.BlogDetail == nil || len(service.gotCreateReq.BlogDetail.Sections) != 1 {
		t.Fatalf("expected blog detail section, got %#v", service.gotCreateReq)
	}
	if service.gotCreateReq.BlogDetail.Sections[0].Image == nil ||
		service.gotCreateReq.BlogDetail.Sections[0].Image.Asset == nil ||
		string(service.gotCreateReq.BlogDetail.Sections[0].Image.Asset.Content) != "story-bytes" {
		t.Fatalf("expected section image bytes to be forwarded, got %#v", service.gotCreateReq.BlogDetail.Sections[0])
	}
}

func TestCreateBlogEndpointAcceptsMultipartAnimationImageUpload(t *testing.T) {
	service := &fakeBlogService{}
	router := setupProtectedBlogRouter(service)

	req := newBlogMultipartRequest(
		t,
		http.MethodPost,
		"/api/blogs",
		`{"publish_date":"2026-06-19","heading":"Tree Planting","description":"Story summary","cover_image":{"file_url":"gs://bucket/blogs/1/cover.png","gcp_object_key":"blogs/1/cover.png"},"blog_detail":{"sections":[{"section_name":"Animation","section_type":"animation","sort_order":0,"is_enabled":true,"settings":{},"animation":{"navigation":"vertical","image_position":"left","items":[{"sort_order":0,"heading":"First Root","sub_heading":"1981","description":"Detail text"}]}}]}}`,
		map[string]multipartUploadTestFile{
			"blog_detail.sections[0].animation.items[0].image.file": {Filename: "frame.png", Data: []byte("frame-bytes")},
		},
	)
	req.Header.Set("Authorization", "Bearer "+signedBlogTestToken(t, "test-secret", 7))

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	itemImage := service.gotCreateReq.BlogDetail.Sections[0].Animation.Items[0].Image
	if itemImage == nil || string(itemImage.Content) != "frame-bytes" {
		t.Fatalf("expected animation item image bytes to be forwarded, got %#v", itemImage)
	}
}

func TestCreateBlogEndpointRejectsBase64UploadPayload(t *testing.T) {
	service := &fakeBlogService{}
	router := setupProtectedBlogRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/blogs",
		strings.NewReader(`{"publish_date":"2026-06-19","heading":"Story","description":"Summary","cover_image":{"file_name":"cover.png","mime_type":"image/png","data_base64":"aGVsbG8="}}`),
	)
	req.Header.Set("Authorization", "Bearer "+signedBlogTestToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertBlogAPIError(t, res, http.StatusBadRequest, "validation_error", "use multipart/form-data with a payload field for file uploads")
}

func TestDeleteBlogEndpointAndErrors(t *testing.T) {
	service := &fakeBlogService{}
	router := setupProtectedBlogRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/blogs/44", nil)
	req.Header.Set("Authorization", "Bearer "+signedBlogTestToken(t, "test-secret", 7))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDeleteID != 44 {
		t.Fatalf("expected delete id 44, got %d", service.gotDeleteID)
	}

	router = setupBlogRouter(&fakeBlogService{detailErr: ErrBlogNotFound})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/blogs/99", nil)
	router.ServeHTTP(res, req)
	assertBlogAPIError(t, res, http.StatusNotFound, "not_found", "blog not found")

	router = setupBlogRouter(&fakeBlogService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/blogs/bad", nil)
	router.ServeHTTP(res, req)
	payload := assertBlogAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
		t.Fatalf("expected id validation detail, got %#v", payload)
	}

	protected := setupProtectedBlogRouter(&fakeBlogService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/blogs", strings.NewReader(`{"heading":"Story"}`))
	req.Header.Set("Content-Type", "application/json")
	protected.ServeHTTP(res, req)
	assertBlogAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writeBlogError(c, ErrMediaBucketNotConfigured)
	assertBlogAPIError(t, rec, http.StatusServiceUnavailable, "service_unavailable", "Blog service is temporarily unavailable")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writeBlogError(c, errors.New("heading is required"))
	assertBlogAPIError(t, rec, http.StatusBadRequest, "validation_error", "heading is required")
}

func assertBlogAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

func newBlogMultipartRequest(t *testing.T, method string, target string, payload string, files map[string]multipartUploadTestFile) *http.Request {
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

func signedBlogTestToken(t *testing.T, secret string, userID int) string {
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

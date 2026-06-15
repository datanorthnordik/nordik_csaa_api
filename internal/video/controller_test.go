package video

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

type fakeVideoService struct {
	listResp       *VideoPackageListResponse
	detailResp     *VideoPackageDetailResponse
	itemResp       *VideoItemResponse
	createResp     *VideoPackageMutationResponse
	updateResp     *VideoPackageMutationResponse
	addResp        *AddVideoItemsResponse
	deleteItemResp *DeleteVideoItemResponse
	mediaResp      *VideoMediaContent
	listErr        error
	detailErr      error
	itemErr        error
	createErr      error
	updateErr      error
	deleteErr      error
	addErr         error
	deleteItemErr  error
	mediaErr       error
	gotCreateReq   SaveVideoPackageRequest
	gotUpdateReq   UpdateVideoPackageRequest
	gotAddReq      AddVideoItemsRequest
	gotItemReq     UpdateVideoItemRequest
	gotID          int
	gotItemID      int
}

func (f *fakeVideoService) ListVideoPackages() (*VideoPackageListResponse, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp == nil {
		return &VideoPackageListResponse{Items: []VideoPackageSummaryItem{}}, nil
	}
	return f.listResp, nil
}

func (f *fakeVideoService) GetVideoPackage(id int) (*VideoPackageDetailResponse, error) {
	f.gotID = id
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if f.detailResp == nil {
		return &VideoPackageDetailResponse{ID: id, Videos: []VideoItemResponse{}}, nil
	}
	return f.detailResp, nil
}

func (f *fakeVideoService) GetVideoTeaserContent(id int, itemID int) (*VideoMediaContent, error) {
	f.gotID = id
	f.gotItemID = itemID
	if f.mediaErr != nil {
		return nil, f.mediaErr
	}
	if f.mediaResp == nil {
		return &VideoMediaContent{Content: []byte("teaser"), ContentType: "image/png", FileName: "teaser.png"}, nil
	}
	return f.mediaResp, nil
}

func (f *fakeVideoService) CreateVideoPackage(req SaveVideoPackageRequest, userID *int) (*VideoPackageMutationResponse, error) {
	f.gotCreateReq = req
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp == nil {
		return &VideoPackageMutationResponse{ID: 1, Title: req.Title, PackageType: req.PackageType}, nil
	}
	return f.createResp, nil
}

func (f *fakeVideoService) UpdateVideoPackage(id int, req UpdateVideoPackageRequest, userID *int) (*VideoPackageMutationResponse, error) {
	f.gotID = id
	f.gotUpdateReq = req
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp == nil {
		return &VideoPackageMutationResponse{ID: id, Title: req.Title, PackageType: VideoPackageTypeCollection}, nil
	}
	return f.updateResp, nil
}

func (f *fakeVideoService) DeleteVideoPackage(id int) error {
	f.gotID = id
	return f.deleteErr
}

func (f *fakeVideoService) AddVideoItems(id int, req AddVideoItemsRequest, userID *int) (*AddVideoItemsResponse, error) {
	f.gotID = id
	f.gotAddReq = req
	if f.addErr != nil {
		return nil, f.addErr
	}
	if f.addResp == nil {
		return &AddVideoItemsResponse{UploadedCount: len(req.Videos)}, nil
	}
	return f.addResp, nil
}

func (f *fakeVideoService) UpdateVideoItem(id int, itemID int, req UpdateVideoItemRequest, userID *int) (*VideoItemResponse, error) {
	f.gotID = id
	f.gotItemID = itemID
	f.gotItemReq = req
	if f.itemErr != nil {
		return nil, f.itemErr
	}
	if f.itemResp == nil {
		return &VideoItemResponse{ID: itemID, VideoPackageID: id, Title: req.Title, YouTubeURL: req.YouTubeURL}, nil
	}
	return f.itemResp, nil
}

func (f *fakeVideoService) DeleteVideoItem(id int, itemID int) (*DeleteVideoItemResponse, error) {
	f.gotID = id
	f.gotItemID = itemID
	if f.deleteItemErr != nil {
		return nil, f.deleteItemErr
	}
	if f.deleteItemResp == nil {
		return &DeleteVideoItemResponse{DeletedCount: 1}, nil
	}
	return f.deleteItemResp, nil
}

func setupVideoRouter(service VideoServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestCreateVideoPackageEndpointAcceptsMultipartSingleVideo(t *testing.T) {
	service := &fakeVideoService{}
	router := setupVideoRouter(service)

	req := newVideoMultipartRequest(t, http.MethodPost, "/api/videos", `{"package_type":"single","single_video":{"title":"Board Update","youtube_url":"https://www.youtube.com/watch?v=abc123","mime_type":"image/png"}}`, map[string]multipartUploadTestFile{
		"single_video.teaser_image_file": {Filename: "teaser.png", Data: []byte("hello")},
	})
	req.Header.Set("Authorization", "Bearer "+signVideoToken(t))

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.SingleVideo == nil || string(service.gotCreateReq.SingleVideo.Content) != "hello" {
		t.Fatalf("expected uploaded teaser bytes, got %#v", service.gotCreateReq.SingleVideo)
	}
}

func TestListGetAndTeaserEndpoints(t *testing.T) {
	service := &fakeVideoService{
		listResp: &VideoPackageListResponse{Items: []VideoPackageSummaryItem{{ID: 7, Title: "Community Videos"}}},
		detailResp: &VideoPackageDetailResponse{
			ID:          7,
			Title:       "Community Videos",
			PackageType: VideoPackageTypeCollection,
			Videos:      []VideoItemResponse{{ID: 21, Title: "Chapter One"}},
		},
	}
	router := setupVideoRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d", res.Code)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/videos/7", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotID != 7 {
		t.Fatalf("expected detail 200 for package 7, got status=%d id=%d", res.Code, service.gotID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/videos/7/items/21/teaser/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotItemID != 21 {
		t.Fatalf("expected teaser content 200 for item 21, got status=%d item=%d", res.Code, service.gotItemID)
	}
}

func TestAddUpdateDeleteVideoItemEndpoints(t *testing.T) {
	service := &fakeVideoService{}
	router := setupVideoRouter(service)

	req := newVideoMultipartRequest(t, http.MethodPost, "/api/videos/7/items", `{"videos":[{"title":"Chapter One","youtube_url":"https://youtu.be/example123","mime_type":"image/png"}]}`, map[string]multipartUploadTestFile{
		"videos[0].teaser_image_file": {Filename: "cover.png", Data: []byte("hello")},
	})
	req.Header.Set("Authorization", "Bearer "+signVideoToken(t))

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || len(service.gotAddReq.Videos) != 1 {
		t.Fatalf("unexpected add item result: status=%d req=%#v", res.Code, service.gotAddReq)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/videos/7/items/21", strings.NewReader(`{"title":"Chapter One Revised","youtube_url":"https://youtu.be/example123","description":"Updated"}`))
	req.Header.Set("Authorization", "Bearer "+signVideoToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotItemID != 21 || service.gotItemReq.Title != "Chapter One Revised" {
		t.Fatalf("unexpected update item result: status=%d req=%#v", res.Code, service.gotItemReq)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/videos/7/items/21", nil)
	req.Header.Set("Authorization", "Bearer "+signVideoToken(t))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotItemID != 21 {
		t.Fatalf("unexpected delete item result: status=%d item=%d", res.Code, service.gotItemID)
	}
}

func TestVideoControllerErrorsAndAuth(t *testing.T) {
	router := setupVideoRouter(&fakeVideoService{createErr: ErrStoreUnavailable})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos", strings.NewReader(`{"title":"Videos"}`))
	req.Header.Set("Authorization", "Bearer "+signVideoToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertVideoError(t, res, http.StatusServiceUnavailable, "service_unavailable")

	router = setupVideoRouter(&fakeVideoService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/videos", strings.NewReader(`{"package_type":"single","single_video":{"title":"Board Update","youtube_url":"https://www.youtube.com/watch?v=abc123","file_name":"teaser.png","mime_type":"image/png","data_base64":"aGVsbG8="}}`))
	req.Header.Set("Authorization", "Bearer "+signVideoToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertVideoError(t, res, http.StatusBadRequest, "validation_error")

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/videos", strings.NewReader(`{"title":"Videos"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertVideoError(t, res, http.StatusUnauthorized, "missing_bearer_token")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writeVideoError(c, errors.New("bad"))
	assertVideoError(t, rec, http.StatusInternalServerError, "internal_error")
}

func assertVideoError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, rec.Code, rec.Body.String())
	}
	var payload apiresponse.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Code != code {
		t.Fatalf("expected code %q, got %#v", code, payload)
	}
}

func signVideoToken(t *testing.T) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": 7,
		"email":   "ada@example.com",
		"role":    "Admin",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

type multipartUploadTestFile struct {
	Filename string
	Data     []byte
}

func newVideoMultipartRequest(t *testing.T, method string, target string, payload string, files map[string]multipartUploadTestFile) *http.Request {
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

package gallery

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

type fakeGalleryService struct {
	listResp               *GalleryListResponse
	detailResp             *GalleryDetailResponse
	imageResp              *GalleryAssetResponse
	createResp             *GalleryMutationResponse
	updateResp             *GalleryMutationResponse
	addImagesResp          *AddGalleryImagesResponse
	deleteImagesResp       *DeleteGalleryImagesResponse
	reorderResp            *ReorderGalleryImagesResponse
	mediaResp              *GalleryMediaContent
	listErr                error
	detailErr              error
	imageErr               error
	createErr              error
	updateErr              error
	deleteErr              error
	addImagesErr           error
	reorderErr             error
	deleteImagesErr        error
	coverErr               error
	imageContentErr        error
	gotList                bool
	gotDetailID            int
	gotCreateReq           SaveGalleryRequest
	gotUpdateID            int
	gotUpdateReq           SaveGalleryRequest
	gotDeleteID            int
	gotAddID               int
	gotAddReq              AddGalleryImagesRequest
	gotUpdateImgID         int
	gotUpdateImgReq        UpdateGalleryImageRequest
	gotReorderID           int
	gotReorderIDs          []int
	gotCoverID             int
	gotImageContentID      int
	gotImageContentImageID int
	gotDeleteImgID         int
	gotDeleteURLs          []string
	gotUserID              *int
}

func (f *fakeGalleryService) ListGalleries() (*GalleryListResponse, error) {
	f.gotList = true
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp == nil {
		return &GalleryListResponse{}, nil
	}
	return f.listResp, nil
}

func (f *fakeGalleryService) GetGallery(id int) (*GalleryDetailResponse, error) {
	f.gotDetailID = id
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if f.detailResp == nil {
		return &GalleryDetailResponse{ID: id, AssetLimit: 20}, nil
	}
	return f.detailResp, nil
}

func (f *fakeGalleryService) GetGalleryCoverContent(id int) (*GalleryMediaContent, error) {
	f.gotCoverID = id
	if f.coverErr != nil {
		return nil, f.coverErr
	}
	if f.mediaResp == nil {
		return &GalleryMediaContent{Content: []byte("cover"), ContentType: "image/png", FileName: "cover.png"}, nil
	}
	return f.mediaResp, nil
}

func (f *fakeGalleryService) GetGalleryImageContent(id int, imageID int) (*GalleryMediaContent, error) {
	f.gotImageContentID = id
	f.gotImageContentImageID = imageID
	if f.imageContentErr != nil {
		return nil, f.imageContentErr
	}
	if f.mediaResp == nil {
		return &GalleryMediaContent{Content: []byte("image"), ContentType: "image/png", FileName: "image.png"}, nil
	}
	return f.mediaResp, nil
}

func (f *fakeGalleryService) CreateGallery(req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error) {
	f.gotCreateReq = req
	f.gotUserID = userID
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp == nil {
		return &GalleryMutationResponse{ID: 1, Name: req.Name, Published: req.Published}, nil
	}
	return f.createResp, nil
}

func (f *fakeGalleryService) UpdateGallery(id int, req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error) {
	f.gotUpdateID = id
	f.gotUpdateReq = req
	f.gotUserID = userID
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp == nil {
		return &GalleryMutationResponse{ID: id, Name: req.Name, Published: req.Published}, nil
	}
	return f.updateResp, nil
}

func (f *fakeGalleryService) DeleteGallery(id int) error {
	f.gotDeleteID = id
	return f.deleteErr
}

func (f *fakeGalleryService) AddGalleryImages(id int, req AddGalleryImagesRequest, userID *int) (*AddGalleryImagesResponse, error) {
	f.gotAddID = id
	f.gotAddReq = req
	f.gotUserID = userID
	if f.addImagesErr != nil {
		return nil, f.addImagesErr
	}
	if f.addImagesResp == nil {
		return &AddGalleryImagesResponse{UploadedCount: len(req.Images)}, nil
	}
	return f.addImagesResp, nil
}

func (f *fakeGalleryService) UpdateGalleryImage(id int, imageID int, req UpdateGalleryImageRequest) (*GalleryAssetResponse, error) {
	f.gotUpdateID = id
	f.gotUpdateImgID = imageID
	f.gotUpdateImgReq = req
	if f.imageErr != nil {
		return nil, f.imageErr
	}
	if f.imageResp == nil {
		return &GalleryAssetResponse{ID: imageID, GalleryID: id, Title: req.Title, AltText: req.AltText}, nil
	}
	return f.imageResp, nil
}

func (f *fakeGalleryService) ReorderGalleryImages(id int, imageIDs []int) (*ReorderGalleryImagesResponse, error) {
	f.gotReorderID = id
	f.gotReorderIDs = imageIDs
	if f.reorderErr != nil {
		return nil, f.reorderErr
	}
	if f.reorderResp == nil {
		return &ReorderGalleryImagesResponse{UpdatedCount: len(imageIDs)}, nil
	}
	return f.reorderResp, nil
}

func (f *fakeGalleryService) DeleteGalleryImages(id int, storageURLs []string) (*DeleteGalleryImagesResponse, error) {
	f.gotDeleteImgID = id
	f.gotDeleteURLs = storageURLs
	if f.deleteImagesErr != nil {
		return nil, f.deleteImagesErr
	}
	if f.deleteImagesResp == nil {
		return &DeleteGalleryImagesResponse{DeletedCount: len(storageURLs)}, nil
	}
	return f.deleteImagesResp, nil
}

func setupProtectedRouter(service GalleryServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func setupRouter(service GalleryServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestCreateGalleryEndpoint(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupProtectedRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/galleries", strings.NewReader(`{"name":"Homepage","description":"Hero gallery","published":true}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.Name != "Homepage" {
		t.Fatalf("unexpected create req: %#v", service.gotCreateReq)
	}
}

func TestCreateGalleryEndpointAcceptsMultipartCoverUpload(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupProtectedRouter(service)

	req := newGalleryMultipartRequest(t, http.MethodPost, "/api/galleries", `{"name":"Homepage","description":"Hero gallery","published":true}`, map[string]multipartUploadTestFile{
		"cover_image_file": {Filename: "cover.png", Data: []byte("hello")},
	})
	req.Header.Set("Authorization", "Bearer "+signToken(t))

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.CoverImage == nil {
		t.Fatalf("expected cover image in request, got %#v", service.gotCreateReq)
	}
	if string(service.gotCreateReq.CoverImage.Content) != "hello" {
		t.Fatalf("expected uploaded cover bytes, got %#v", service.gotCreateReq.CoverImage)
	}
}

func TestListAndGetGalleryEndpoints(t *testing.T) {
	service := &fakeGalleryService{
		listResp: &GalleryListResponse{
			Items: []GallerySummaryItem{{ID: 4, Name: "Homepage", AssetCount: 2}},
		},
		detailResp: &GalleryDetailResponse{ID: 4, Name: "Homepage", AssetLimit: 20},
	}
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/galleries", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !service.gotList {
		t.Fatalf("unexpected list result: status=%d gotList=%v", res.Code, service.gotList)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/galleries/4", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotDetailID != 4 {
		t.Fatalf("unexpected detail result: status=%d id=%d", res.Code, service.gotDetailID)
	}
}

func TestUpdateGalleryEndpoint(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupProtectedRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/galleries/7", strings.NewReader(`{"name":"Updated","description":"New"}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK || service.gotUpdateID != 7 {
		t.Fatalf("unexpected update result: status=%d id=%d", res.Code, service.gotUpdateID)
	}
}

func TestDeleteGalleryEndpoint(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupProtectedRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/galleries/9", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK || service.gotDeleteID != 9 {
		t.Fatalf("unexpected delete result: status=%d id=%d", res.Code, service.gotDeleteID)
	}
}

func TestGalleryContentEndpoints(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/galleries/4/cover/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotCoverID != 4 {
		t.Fatalf("unexpected cover result: status=%d id=%d", res.Code, service.gotCoverID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/galleries/4/images/12/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotImageContentID != 4 || service.gotImageContentImageID != 12 {
		t.Fatalf(
			"unexpected image content result: status=%d gallery=%d image=%d",
			res.Code,
			service.gotImageContentID,
			service.gotImageContentImageID,
		)
	}
}

func TestGalleryMutationEndpointsStillRequireAuth(t *testing.T) {
	router := setupRouter(&fakeGalleryService{})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/galleries", strings.NewReader(`{"name":"Homepage"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated mutation, got %d: %s", res.Code, res.Body.String())
	}
}

func TestAddAndDeleteGalleryImagesEndpoints(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupProtectedRouter(service)

	res := httptest.NewRecorder()
	req := newGalleryMultipartRequest(t, http.MethodPost, "/api/galleries/4/images", `{"images":[{"title":"Opening banner","alt_text":"Banner","link_url":"https://partner.example.com","mime_type":"image/png"}]}`, map[string]multipartUploadTestFile{
		"images[0].file": {Filename: "banner.png", Data: []byte("hello")},
	})
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || service.gotAddID != 4 || len(service.gotAddReq.Images) != 1 {
		t.Fatalf("unexpected add image result: status=%d req=%#v", res.Code, service.gotAddReq)
	}
	if service.gotAddReq.Images[0].Title != "Opening banner" {
		t.Fatalf("expected image title to be forwarded, got %#v", service.gotAddReq.Images[0])
	}
	if service.gotAddReq.Images[0].LinkURL != "https://partner.example.com" {
		t.Fatalf("expected image link_url to be forwarded, got %#v", service.gotAddReq.Images[0])
	}
	if string(service.gotAddReq.Images[0].Content) != "hello" {
		t.Fatalf("expected uploaded image bytes, got %#v", service.gotAddReq.Images[0])
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/galleries/4/images?storage_url=gs://bucket/galleries/4/images/a.png&storage_url=gs://bucket/galleries/4/images/b.png", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || len(service.gotDeleteURLs) != 2 {
		t.Fatalf("unexpected delete image result: status=%d urls=%#v", res.Code, service.gotDeleteURLs)
	}
}

func TestUpdateAndReorderGalleryImagesEndpoints(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupProtectedRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/galleries/4/images/12", strings.NewReader(`{"title":"Opening banner","alt_text":"Banner details","link_url":"https://partner.example.com"}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotUpdateID != 4 || service.gotUpdateImgID != 12 {
		t.Fatalf(
			"unexpected update image result: status=%d gallery=%d image=%d",
			res.Code,
			service.gotUpdateID,
			service.gotUpdateImgID,
		)
	}
	if service.gotUpdateImgReq.LinkURL != "https://partner.example.com" {
		t.Fatalf("expected update image link_url to be forwarded, got %#v", service.gotUpdateImgReq)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/galleries/4/images/order", strings.NewReader(`{"image_ids":[12,10,11]}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotReorderID != 4 || len(service.gotReorderIDs) != 3 {
		t.Fatalf(
			"unexpected reorder result: status=%d gallery=%d ids=%#v",
			res.Code,
			service.gotReorderID,
			service.gotReorderIDs,
		)
	}
}

func TestGalleryControllerErrors(t *testing.T) {
	router := setupProtectedRouter(&fakeGalleryService{createErr: ErrStoreUnavailable})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/galleries", strings.NewReader(`{"name":"Homepage"}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertError(t, res, http.StatusServiceUnavailable, "service_unavailable")

	router = setupProtectedRouter(&fakeGalleryService{deleteErr: ErrGalleryNotFound})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/galleries/99", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	router.ServeHTTP(res, req)
	assertError(t, res, http.StatusNotFound, "not_found")

	router = setupProtectedRouter(&fakeGalleryService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/galleries/4/images", strings.NewReader(`{"storage_urls":[]}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	assertError(t, res, http.StatusBadRequest, "validation_error")

	router = setupProtectedRouter(&fakeGalleryService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/galleries/bad", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	router.ServeHTTP(res, req)
	assertError(t, res, http.StatusBadRequest, "validation_error")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writeGalleryError(c, errors.New("bad"))
	assertError(t, rec, http.StatusInternalServerError, "internal_error")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writeGalleryError(c, errors.New(`ERROR: duplicate key value violates unique constraint "galleries_name_key" (SQLSTATE 23505)`))
	assertError(t, rec, http.StatusConflict, "conflict")
}

func TestGalleryUploadEndpointsRejectBase64Payload(t *testing.T) {
	router := setupProtectedRouter(&fakeGalleryService{})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/galleries/4/images", strings.NewReader(`{"images":[{"title":"Opening banner","alt_text":"Banner","file_name":"banner.png","mime_type":"image/png","data_base64":"aGVsbG8="}]}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	assertError(t, res, http.StatusBadRequest, "validation_error")
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
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

func signToken(t *testing.T) string {
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

func newGalleryMultipartRequest(t *testing.T, method string, target string, payload string, files map[string]multipartUploadTestFile) *http.Request {
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

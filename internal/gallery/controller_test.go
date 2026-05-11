package gallery

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

type fakeGalleryService struct {
	createResp       *GalleryMutationResponse
	updateResp       *GalleryMutationResponse
	deleteImagesResp *DeleteGalleryImagesResponse
	createErr        error
	updateErr        error
	deleteErr        error
	addImagesErr     error
	deleteImagesErr  error
	gotCreateReq     SaveGalleryRequest
	gotUpdateID      int
	gotUpdateReq     SaveGalleryRequest
	gotDeleteID      int
	gotAddID         int
	gotAddReq        AddGalleryImagesRequest
	gotDeleteImgID   int
	gotDeleteURLs    []string
	gotUserID        *int
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

func (f *fakeGalleryService) AddGalleryImages(id int, req AddGalleryImagesRequest, userID *int) (*DeleteGalleryImagesResponse, error) {
	f.gotAddID = id
	f.gotAddReq = req
	f.gotUserID = userID
	if f.addImagesErr != nil {
		return nil, f.addImagesErr
	}
	return &DeleteGalleryImagesResponse{DeletedCount: len(req.Images)}, nil
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

func TestAddAndDeleteGalleryImagesEndpoints(t *testing.T) {
	service := &fakeGalleryService{}
	router := setupProtectedRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/galleries/4/images", strings.NewReader(`{"images":[{"title":"Opening banner","alt_text":"Banner","file_name":"banner.png","mime_type":"image/png","data_base64":"aGVsbG8="}]}`))
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || service.gotAddID != 4 || len(service.gotAddReq.Images) != 1 {
		t.Fatalf("unexpected add image result: status=%d req=%#v", res.Code, service.gotAddReq)
	}
	if service.gotAddReq.Images[0].Title != "Opening banner" {
		t.Fatalf("expected image title to be forwarded, got %#v", service.gotAddReq.Images[0])
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/galleries/4/images?storage_url=gs://bucket/galleries/4/images/a.png&storage_url=gs://bucket/galleries/4/images/b.png", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || len(service.gotDeleteURLs) != 2 {
		t.Fatalf("unexpected delete image result: status=%d urls=%#v", res.Code, service.gotDeleteURLs)
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

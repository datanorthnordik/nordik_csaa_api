package resources

import (
	"encoding/json"
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

type fakeResourceService struct {
	listResp        *ResourceListResponse
	detailResp      *ResourceDetailResponse
	contentResp     *ResourceContent
	createResp      *ResourceMutationResponse
	updateResp      *ResourceMutationResponse
	listErr         error
	detailErr       error
	contentErr      error
	createErr       error
	updateErr       error
	deleteErr       error
	gotListFilter   ListResourcesFilter
	gotDetailID     int
	gotContentID    int
	gotCreateReq    SaveResourceRequest
	gotCreateUserID *int
	gotUpdateID     int
	gotUpdateReq    SaveResourceRequest
	gotUpdateUserID *int
	gotDeleteID     int
}

func (f *fakeResourceService) ListResources(filter ListResourcesFilter) (*ResourceListResponse, error) {
	f.gotListFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp == nil {
		return &ResourceListResponse{}, nil
	}
	return f.listResp, nil
}

func (f *fakeResourceService) GetResource(id int) (*ResourceDetailResponse, error) {
	f.gotDetailID = id
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if f.detailResp == nil {
		return &ResourceDetailResponse{ID: id, Name: "Resource Guide"}, nil
	}
	return f.detailResp, nil
}

func (f *fakeResourceService) GetResourceContent(id int) (*ResourceContent, error) {
	f.gotContentID = id
	if f.contentErr != nil {
		return nil, f.contentErr
	}
	if f.contentResp == nil {
		return &ResourceContent{Content: []byte("resource"), ContentType: "application/pdf", FileName: "guide.pdf"}, nil
	}
	return f.contentResp, nil
}

func (f *fakeResourceService) CreateResource(req SaveResourceRequest, userID *int) (*ResourceMutationResponse, error) {
	f.gotCreateReq = req
	f.gotCreateUserID = userID
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp == nil {
		return &ResourceMutationResponse{ID: 1, Name: req.Name, Description: req.Description, Category: req.Category, Visibility: req.Visibility, LinkURL: req.LinkURL}, nil
	}
	return f.createResp, nil
}

func (f *fakeResourceService) UpdateResource(id int, req SaveResourceRequest, userID *int) (*ResourceMutationResponse, error) {
	f.gotUpdateID = id
	f.gotUpdateReq = req
	f.gotUpdateUserID = userID
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp == nil {
		return &ResourceMutationResponse{ID: id, Name: req.Name, Description: req.Description, Category: req.Category, Visibility: req.Visibility, LinkURL: req.LinkURL}, nil
	}
	return f.updateResp, nil
}

func (f *fakeResourceService) DeleteResource(id int) error {
	f.gotDeleteID = id
	return f.deleteErr
}

func setupResourceRouter(service ResourceServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestResourceReadEndpointsArePublic(t *testing.T) {
	service := &fakeResourceService{
		listResp: &ResourceListResponse{
			Items: []ResourceListItem{{ID: 9, Name: "Resource Guide", Category: "media", Visibility: "public"}},
		},
		detailResp:  &ResourceDetailResponse{ID: 9, Name: "Resource Guide", Category: "media", Visibility: "public"},
		contentResp: &ResourceContent{Content: []byte("file"), ContentType: "application/pdf", FileName: "guide.pdf"},
	}
	router := setupResourceRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?page=2&page_size=5&search=guide&category=media&file_type=pdf", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.Page != 2 || service.gotListFilter.PageSize != 5 {
		t.Fatalf("unexpected pagination filter: %#v", service.gotListFilter)
	}
	if service.gotListFilter.SearchTerm != "guide" || service.gotListFilter.Category != "media" || service.gotListFilter.FileType != "pdf" {
		t.Fatalf("unexpected list filter: %#v", service.gotListFilter)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/resources/9", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDetailID != 9 {
		t.Fatalf("expected detail id 9, got %d", service.gotDetailID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/resources/9/content", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if res.Body.String() != "file" {
		t.Fatalf("unexpected content body: %q", res.Body.String())
	}
	if service.gotContentID != 9 {
		t.Fatalf("expected content id 9, got %d", service.gotContentID)
	}
	if got := res.Header().Get("Content-Disposition"); !strings.Contains(got, "guide.pdf") {
		t.Fatalf("expected content disposition filename, got %q", got)
	}
}

func TestResourceMutationEndpointsRequireAuth(t *testing.T) {
	router := setupResourceRouter(&fakeResourceService{})

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/resources", body: `{"name":"Guide","description":"Desc","category":"link","visibility":"public","link_url":"https://example.com"}`},
		{method: http.MethodPut, path: "/api/resources/9", body: `{"name":"Guide","description":"Desc","category":"link","visibility":"public","link_url":"https://example.com"}`},
		{method: http.MethodDelete, path: "/api/resources/9"},
	}

	for _, tc := range tests {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(res, req)
		assertResourceAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")
	}
}

func TestResourceMutationEndpointsUseAuthenticatedUserID(t *testing.T) {
	service := &fakeResourceService{}
	router := setupResourceRouter(service)

	createBody := `{"name":"Guide","description":"Desc","category":"link","visibility":"public","link_url":"https://example.com"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources", strings.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+signResourceToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateUserID == nil || *service.gotCreateUserID != 7 {
		t.Fatalf("expected create auth user id 7, got %#v", service.gotCreateUserID)
	}
	if service.gotCreateReq.LinkURL != "https://example.com" {
		t.Fatalf("unexpected create request: %#v", service.gotCreateReq)
	}

	updateBody := `{"name":"Updated Guide","description":"Desc","category":"link","visibility":"public","link_url":"https://example.com/updated"}`
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/resources/9", strings.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+signResourceToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 9 {
		t.Fatalf("expected update id 9, got %d", service.gotUpdateID)
	}
	if service.gotUpdateUserID == nil || *service.gotUpdateUserID != 7 {
		t.Fatalf("expected update auth user id 7, got %#v", service.gotUpdateUserID)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/resources/9", nil)
	req.Header.Set("Authorization", "Bearer "+signResourceToken(t, "test-secret", 7))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDeleteID != 9 {
		t.Fatalf("expected delete id 9, got %d", service.gotDeleteID)
	}
}

func assertResourceAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

func signResourceToken(t *testing.T, secret string, userID int) string {
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

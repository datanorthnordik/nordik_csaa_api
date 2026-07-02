package bookshelf

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

type fakeBookshelfService struct {
	listResp          *BookshelfListResponse
	detailResp        *BookshelfDetailResponse
	bookContentResp   *BookshelfContent
	authorContentResp *BookshelfContent
	coverContentResp  *BookshelfContent
	createResp        *BookshelfMutationResponse
	updateResp        *BookshelfMutationResponse
	listErr           error
	detailErr         error
	bookContentErr    error
	authorContentErr  error
	coverContentErr   error
	createErr         error
	updateErr         error
	deleteErr         error
	gotListFilter     ListBookshelfFilter
	gotDetailID       int
	gotBookID         int
	gotAuthorID       int
	gotCoverID        int
	gotCreateReq      SaveBookshelfEntryRequest
	gotCreateUserID   *int
	gotUpdateID       int
	gotUpdateReq      SaveBookshelfEntryRequest
	gotUpdateUserID   *int
	gotDeleteID       int
}

func (f *fakeBookshelfService) ListBooks(filter ListBookshelfFilter) (*BookshelfListResponse, error) {
	f.gotListFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp == nil {
		return &BookshelfListResponse{}, nil
	}
	return f.listResp, nil
}

func (f *fakeBookshelfService) GetBook(id int) (*BookshelfDetailResponse, error) {
	f.gotDetailID = id
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if f.detailResp == nil {
		return &BookshelfDetailResponse{ID: id, Author: "Author", Title: "Book"}, nil
	}
	return f.detailResp, nil
}

func (f *fakeBookshelfService) GetBookContent(id int) (*BookshelfContent, error) {
	f.gotBookID = id
	if f.bookContentErr != nil {
		return nil, f.bookContentErr
	}
	if f.bookContentResp == nil {
		return &BookshelfContent{Content: []byte("book"), ContentType: "application/pdf", FileName: "book.pdf"}, nil
	}
	return f.bookContentResp, nil
}

func (f *fakeBookshelfService) GetAuthorImageContent(id int) (*BookshelfContent, error) {
	f.gotAuthorID = id
	if f.authorContentErr != nil {
		return nil, f.authorContentErr
	}
	if f.authorContentResp == nil {
		return &BookshelfContent{Content: []byte("author"), ContentType: "image/png", FileName: "author.png"}, nil
	}
	return f.authorContentResp, nil
}

func (f *fakeBookshelfService) GetCoverImageContent(id int) (*BookshelfContent, error) {
	f.gotCoverID = id
	if f.coverContentErr != nil {
		return nil, f.coverContentErr
	}
	if f.coverContentResp == nil {
		return &BookshelfContent{Content: []byte("cover"), ContentType: "image/png", FileName: "cover.png"}, nil
	}
	return f.coverContentResp, nil
}

func (f *fakeBookshelfService) CreateBook(req SaveBookshelfEntryRequest, userID *int) (*BookshelfMutationResponse, error) {
	f.gotCreateReq = req
	f.gotCreateUserID = userID
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp == nil {
		return &BookshelfMutationResponse{ID: 1, Author: req.Author, Title: req.Title}, nil
	}
	return f.createResp, nil
}

func (f *fakeBookshelfService) UpdateBook(id int, req SaveBookshelfEntryRequest, userID *int) (*BookshelfMutationResponse, error) {
	f.gotUpdateID = id
	f.gotUpdateReq = req
	f.gotUpdateUserID = userID
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp == nil {
		return &BookshelfMutationResponse{ID: id, Author: req.Author, Title: req.Title}, nil
	}
	return f.updateResp, nil
}

func (f *fakeBookshelfService) DeleteBook(id int) error {
	f.gotDeleteID = id
	return f.deleteErr
}

func setupBookshelfRouter(service BookshelfServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestBookshelfReadEndpointsArePublic(t *testing.T) {
	service := &fakeBookshelfService{
		listResp: &BookshelfListResponse{
			Items: []BookshelfListItem{{ID: 9, Author: "Author", Title: "Book"}},
		},
		detailResp:        &BookshelfDetailResponse{ID: 9, Author: "Author", Title: "Book"},
		bookContentResp:   &BookshelfContent{Content: []byte("file"), ContentType: "application/pdf", FileName: "book.pdf"},
		authorContentResp: &BookshelfContent{Content: []byte("author"), ContentType: "image/png", FileName: "author.png"},
		coverContentResp:  &BookshelfContent{Content: []byte("img"), ContentType: "image/png", FileName: "cover.png"},
	}
	router := setupBookshelfRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/bookshelf?page=2&page_size=5&search=guide", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.Page != 2 || service.gotListFilter.PageSize != 5 || service.gotListFilter.SearchTerm != "guide" {
		t.Fatalf("unexpected list filter: %#v", service.gotListFilter)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/bookshelf/9", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || service.gotDetailID != 9 {
		t.Fatalf("unexpected detail response: status=%d id=%d body=%s", res.Code, service.gotDetailID, res.Body.String())
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/bookshelf/9/book/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "file" || service.gotBookID != 9 {
		t.Fatalf("unexpected book content response: status=%d id=%d body=%q", res.Code, service.gotBookID, res.Body.String())
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/bookshelf/9/author-image/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "author" || service.gotAuthorID != 9 {
		t.Fatalf("unexpected author image response: status=%d id=%d body=%q", res.Code, service.gotAuthorID, res.Body.String())
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/bookshelf/9/cover/content", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "img" || service.gotCoverID != 9 {
		t.Fatalf("unexpected cover content response: status=%d id=%d body=%q", res.Code, service.gotCoverID, res.Body.String())
	}
}

func TestBookshelfMutationEndpointsRequireAuth(t *testing.T) {
	router := setupBookshelfRouter(&fakeBookshelfService{})

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/bookshelf", body: `{"author":"Author","title":"Book","description":"Desc"}`},
		{method: http.MethodPut, path: "/api/bookshelf/9", body: `{"author":"Author","title":"Book","description":"Desc"}`},
		{method: http.MethodDelete, path: "/api/bookshelf/9"},
	}

	for _, tc := range tests {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(res, req)
		assertBookshelfAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")
	}
}

func TestBookshelfMutationEndpointsUseAuthenticatedUserID(t *testing.T) {
	service := &fakeBookshelfService{}
	router := setupBookshelfRouter(service)

	createBody := `{"author":"Author","title":"Book","book_link":"https://example.com/book","description":"Desc"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/bookshelf", strings.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+signBookshelfToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateUserID == nil || *service.gotCreateUserID != 7 {
		t.Fatalf("expected create auth user id 7, got %#v", service.gotCreateUserID)
	}

	updateBody := `{"author":"Updated Author","title":"Updated Book","description":"Desc","remove_author_image":true,"remove_cover_image":true}`
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/bookshelf/9", strings.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+signBookshelfToken(t, "test-secret", 7))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 9 || service.gotUpdateUserID == nil || *service.gotUpdateUserID != 7 || !service.gotUpdateReq.RemoveCoverImage || !service.gotUpdateReq.RemoveAuthorImage {
		t.Fatalf("unexpected update capture: id=%d user=%#v req=%#v", service.gotUpdateID, service.gotUpdateUserID, service.gotUpdateReq)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/bookshelf/9", nil)
	req.Header.Set("Authorization", "Bearer "+signBookshelfToken(t, "test-secret", 7))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK || service.gotDeleteID != 9 {
		t.Fatalf("unexpected delete response: status=%d id=%d body=%s", res.Code, service.gotDeleteID, res.Body.String())
	}
}

func assertBookshelfAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

func signBookshelfToken(t *testing.T, secret string, userID int) string {
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

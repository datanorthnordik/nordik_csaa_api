package knowledgecenter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authpkg "nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type fakeKnowledgeCenterService struct {
	listResp              *KnowledgeCenterSubmissionsListResponse
	submissionResp        *KnowledgeCenterSubmissionResponse
	listErr               error
	getErr                error
	createErr             error
	completeErr           error
	gotListFilter         ListKnowledgeCenterSubmissionsFilter
	gotGetID              int
	gotCreateReq          CreateKnowledgeCenterSubmissionRequest
	gotCompleteID         int
	gotCompleteReq        CompleteKnowledgeCenterSubmissionRequest
	gotCompleteReviewerID *int
}

func (f *fakeKnowledgeCenterService) ListSubmissions(filter ListKnowledgeCenterSubmissionsFilter) (*KnowledgeCenterSubmissionsListResponse, error) {
	f.gotListFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp == nil {
		return &KnowledgeCenterSubmissionsListResponse{}, nil
	}
	return f.listResp, nil
}

func (f *fakeKnowledgeCenterService) GetSubmission(id int) (*KnowledgeCenterSubmissionResponse, error) {
	f.gotGetID = id
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.submissionResp == nil {
		return &KnowledgeCenterSubmissionResponse{ID: id}, nil
	}
	return f.submissionResp, nil
}

func (f *fakeKnowledgeCenterService) CreatePublicSubmission(req CreateKnowledgeCenterSubmissionRequest) (*KnowledgeCenterSubmissionResponse, error) {
	f.gotCreateReq = req
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.submissionResp == nil {
		return &KnowledgeCenterSubmissionResponse{ID: 1, SubmitterName: req.SubmitterName}, nil
	}
	return f.submissionResp, nil
}

func (f *fakeKnowledgeCenterService) MarkSubmissionCompleted(id int, req CompleteKnowledgeCenterSubmissionRequest, userID *int) (*KnowledgeCenterSubmissionResponse, error) {
	f.gotCompleteID = id
	f.gotCompleteReq = req
	f.gotCompleteReviewerID = userID
	if f.completeErr != nil {
		return nil, f.completeErr
	}
	if f.submissionResp == nil {
		return &KnowledgeCenterSubmissionResponse{ID: id, Status: KnowledgeCenterSubmissionStatusCompleted}, nil
	}
	return f.submissionResp, nil
}

func setupKnowledgeCenterRouter(service KnowledgeCenterServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, service)
	return router
}

func setupProtectedKnowledgeCenterRouter(service KnowledgeCenterServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return router
}

func TestKnowledgeCenterCreatePublicSubmission(t *testing.T) {
	service := &fakeKnowledgeCenterService{}
	router := setupKnowledgeCenterRouter(service)

	body := bytes.NewBufferString(`{"name":"Alice","email":"alice@example.com","phone":"555-1234","type":"video","message":"I have a clip to share."}`)
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-center/submissions", body)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateReq.SubmitterName != "Alice" || service.gotCreateReq.SubmitterEmail != "alice@example.com" {
		t.Fatalf("unexpected create request: %#v", service.gotCreateReq)
	}
}

func TestKnowledgeCenterListSubmissionsParsesFilter(t *testing.T) {
	service := &fakeKnowledgeCenterService{
		listResp: &KnowledgeCenterSubmissionsListResponse{
			Items: []KnowledgeCenterSubmissionResponse{{ID: 4}},
		},
	}
	router := setupProtectedKnowledgeCenterRouter(service)

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-center/submissions?page=2&page_size=15&search=alice&status=completed", nil)
	req.Header.Set("Authorization", "Bearer "+signedTestKnowledgeCenterToken(t, 7))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.Page != 2 || service.gotListFilter.PageSize != 15 || service.gotListFilter.SearchTerm != "alice" || service.gotListFilter.Status != "completed" {
		t.Fatalf("unexpected filter: %#v", service.gotListFilter)
	}
}

func TestKnowledgeCenterMarkSubmissionCompletedUsesAuthUserID(t *testing.T) {
	service := &fakeKnowledgeCenterService{}
	router := setupProtectedKnowledgeCenterRouter(service)

	body := bytes.NewBufferString(`{"completion_notes":"Uploaded and published."}`)
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-center/submissions/9/complete", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signedTestKnowledgeCenterToken(t, 41))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCompleteID != 9 {
		t.Fatalf("expected submission id 9, got %d", service.gotCompleteID)
	}
	if service.gotCompleteReq.CompletionNotes != "Uploaded and published." {
		t.Fatalf("unexpected completion request: %#v", service.gotCompleteReq)
	}
	if service.gotCompleteReviewerID == nil || *service.gotCompleteReviewerID != 41 {
		t.Fatalf("expected reviewer id 41, got %#v", service.gotCompleteReviewerID)
	}
}

func signedTestKnowledgeCenterToken(t *testing.T, userID int) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   "reviewer@example.com",
		"role":    "admin",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	})

	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestKnowledgeCenterGetSubmission(t *testing.T) {
	service := &fakeKnowledgeCenterService{
		submissionResp: &KnowledgeCenterSubmissionResponse{ID: 12, SubmitterName: "Alice"},
	}
	router := setupProtectedKnowledgeCenterRouter(service)

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-center/submissions/12", nil)
	req.Header.Set("Authorization", "Bearer "+signedTestKnowledgeCenterToken(t, 1))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload struct {
		Submission KnowledgeCenterSubmissionResponse `json:"submission"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Submission.ID != 12 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

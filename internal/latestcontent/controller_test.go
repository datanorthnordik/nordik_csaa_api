package latestcontent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeService struct {
	limit int
}

func (service *fakeService) ListLatestContent(limit int) (*LatestContentResponse, error) {
	service.limit = limit
	return &LatestContentResponse{Items: []LatestContentItem{}}, nil
}

func TestListLatestContentUsesRequestedBoundedLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeService{}
	router := gin.New()
	RegisterRoutes(router, service)

	request := httptest.NewRequest(http.MethodGet, "/api/latest-content?limit=3", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if service.limit != 3 {
		t.Fatalf("expected limit 3, got %d", service.limit)
	}

	var body LatestContentResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Items == nil {
		t.Fatal("expected items to serialize as an empty array")
	}
}

func TestListLatestContentRejectsLimitsAboveRetentionSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, &fakeService{})

	request := httptest.NewRequest(http.MethodGet, "/api/latest-content?limit=11", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

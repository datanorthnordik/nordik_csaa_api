package events

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

type fakeEventService struct {
	listResp               *EventListResponse
	eventResp              *EventDetailResponse
	locationsResp          *SavedLocationListResponse
	galleriesResp          *GalleryListResponse
	createResp             *EventMutationResponse
	updateResp             *EventMutationResponse
	deleteAllResp          *DeleteAllDocumentsResponse
	mediaContentResp       *EventMediaContent
	listErr                error
	eventErr               error
	locationsErr           error
	galleriesErr           error
	createErr              error
	updateErr              error
	mediaContentErr        error
	deleteErr              error
	deleteDocumentErr      error
	deleteAllErr           error
	deletePhotoErr         error
	gotListFilter          ListEventsFilter
	gotCreateRequest       SaveEventRequest
	gotGetID               int
	gotMediaContentID      int
	gotMediaContentMediaID int
	gotUpdateID            int
	gotUpdateRequest       SaveEventRequest
	gotDeleteID            int
	gotDeleteDocumentID    int
	gotDeleteMediaID       int
	gotDeleteAllID         int
	gotDeletePhotoID       int
	gotDeletePhotoMedia    int
}

func (s *fakeEventService) ListEvents(filter ListEventsFilter) (*EventListResponse, error) {
	s.gotListFilter = filter
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResp == nil {
		return &EventListResponse{}, nil
	}
	return s.listResp, nil
}

func (s *fakeEventService) GetEvent(id int) (*EventDetailResponse, error) {
	s.gotGetID = id
	if s.eventErr != nil {
		return nil, s.eventErr
	}
	if s.eventResp == nil {
		return &EventDetailResponse{ID: id, Title: "Spring Fair"}, nil
	}
	return s.eventResp, nil
}

func (s *fakeEventService) GetEventMediaContent(eventID int, mediaID int) (*EventMediaContent, error) {
	s.gotMediaContentID = eventID
	s.gotMediaContentMediaID = mediaID
	if s.mediaContentErr != nil {
		return nil, s.mediaContentErr
	}
	if s.mediaContentResp == nil {
		return &EventMediaContent{Content: []byte("ok"), ContentType: "application/octet-stream", FileName: "file.bin"}, nil
	}
	return s.mediaContentResp, nil
}

func (s *fakeEventService) ListSavedLocations() (*SavedLocationListResponse, error) {
	if s.locationsErr != nil {
		return nil, s.locationsErr
	}
	if s.locationsResp == nil {
		return &SavedLocationListResponse{}, nil
	}
	return s.locationsResp, nil
}

func (s *fakeEventService) ListGalleries() (*GalleryListResponse, error) {
	if s.galleriesErr != nil {
		return nil, s.galleriesErr
	}
	if s.galleriesResp == nil {
		return &GalleryListResponse{}, nil
	}
	return s.galleriesResp, nil
}

func (s *fakeEventService) CreateEvent(req SaveEventRequest) (*EventMutationResponse, error) {
	s.gotCreateRequest = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createResp == nil {
		return &EventMutationResponse{ID: 1, Title: req.Title, Published: req.Published}, nil
	}
	return s.createResp, nil
}

func (s *fakeEventService) UpdateEvent(id int, req SaveEventRequest) (*EventMutationResponse, error) {
	s.gotUpdateID = id
	s.gotUpdateRequest = req
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.updateResp == nil {
		return &EventMutationResponse{ID: id, Title: req.Title, Published: req.Published}, nil
	}
	return s.updateResp, nil
}

func (s *fakeEventService) DeleteEvent(id int) error {
	s.gotDeleteID = id
	return s.deleteErr
}

func (s *fakeEventService) DeleteEventDocument(id int, mediaID int) error {
	s.gotDeleteDocumentID = id
	s.gotDeleteMediaID = mediaID
	return s.deleteDocumentErr
}

func (s *fakeEventService) DeleteAllEventDocuments(id int) (*DeleteAllDocumentsResponse, error) {
	s.gotDeleteAllID = id
	if s.deleteAllErr != nil {
		return nil, s.deleteAllErr
	}
	if s.deleteAllResp == nil {
		return &DeleteAllDocumentsResponse{DeletedCount: 2}, nil
	}
	return s.deleteAllResp, nil
}

func (s *fakeEventService) DeleteEventPhoto(id int, mediaID int) error {
	s.gotDeletePhotoID = id
	s.gotDeletePhotoMedia = mediaID
	return s.deletePhotoErr
}

func setupEventRouter(service EventServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service)
	return r
}

func setupProtectedEventRouter(service EventServicePort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, service, authpkg.RequireBearerAuth(&config.Config{JWTSecret: "test-secret"}))
	return r
}

func TestListSavedLocationsEndpoint(t *testing.T) {
	service := &fakeEventService{
		locationsResp: &SavedLocationListResponse{
			Items: []Address{
				{ID: 7, Name: "Community Hall", City: "Toronto", Country: "Canada", IsSaved: true},
			},
		},
	}
	router := setupEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/locations", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload SavedLocationListResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != 7 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestListSavedLocationSingularEndpoint(t *testing.T) {
	service := &fakeEventService{
		locationsResp: &SavedLocationListResponse{
			Items: []Address{
				{ID: 8, Name: "Shingwauk Hall", City: "Sault Ste. Marie", Country: "Canada", IsSaved: true},
			},
		},
	}
	router := setupEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/location", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
}

func TestGetEventEndpoint(t *testing.T) {
	service := &fakeEventService{
		eventResp: &EventDetailResponse{
			ID:        12,
			Title:     "Spring Fair",
			EventType: "single_day_partial",
		},
	}
	router := setupEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/12", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload EventDetailResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ID != 12 || payload.Title != "Spring Fair" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if service.gotGetID != 12 {
		t.Fatalf("expected get id 12, got %d", service.gotGetID)
	}
}

func TestGetEventEndpointErrors(t *testing.T) {
	router := setupEventRouter(&fakeEventService{eventErr: ErrEventNotFound})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/99", nil)
	router.ServeHTTP(res, req)

	assertEventAPIError(t, res, http.StatusNotFound, "not_found", "event not found")

	router = setupEventRouter(&fakeEventService{})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/events/bad", nil)
	router.ServeHTTP(res, req)

	payload := assertEventAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
		t.Fatalf("expected id validation detail, got %#v", payload)
	}
}

func TestListSavedLocationsEndpointHandlesError(t *testing.T) {
	router := setupEventRouter(&fakeEventService{locationsErr: ErrStoreUnavailable})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/locations", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Event service is temporarily unavailable")
}

func TestListGalleriesEndpoint(t *testing.T) {
	service := &fakeEventService{
		galleriesResp: &GalleryListResponse{
			Items: []Gallery{
				{ID: 9, Name: "Homepage Gallery"},
			},
		},
	}
	router := setupEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/galleries", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload GalleryListResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != 9 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestListGalleriesEndpointHandlesError(t *testing.T) {
	router := setupEventRouter(&fakeEventService{galleriesErr: ErrStoreUnavailable})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/galleries", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Event service is temporarily unavailable")
}

func TestCreateEventEndpoint(t *testing.T) {
	service := &fakeEventService{}
	router := setupEventRouter(service)

	body := `{"title":"Spring Fair","show_title":true,"categories":["Events"],"event_type":"single_day_all_day","start_at":"2026-05-01T10:00:00Z","privacy_type":"public","teaser":"Welcome!"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateRequest.Title != "Spring Fair" {
		t.Fatalf("expected title to be forwarded, got %q", service.gotCreateRequest.Title)
	}
}

func TestCreateEventEndpointAllowsMissingTeaser(t *testing.T) {
	service := &fakeEventService{}
	router := setupEventRouter(service)

	body := `{"title":"Spring Fair","show_title":true,"categories":["Events"],"event_type":"single_day_all_day","start_at":"2026-05-01T10:00:00Z","privacy_type":"public"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotCreateRequest.Teaser != "" {
		t.Fatalf("expected empty teaser, got %q", service.gotCreateRequest.Teaser)
	}
}

func TestListEventsEndpoint(t *testing.T) {
	service := &fakeEventService{
		listResp: &EventListResponse{
			Items: []EventListItem{
				{ID: 9, Title: "Spring Fair", Status: EventStatusPublished},
			},
			Pagination: EventListPageMeta{Page: 2, PageSize: 5, TotalItems: 11, TotalPages: 3, HasNext: true, HasPrev: true},
		},
	}
	router := setupEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events?page=2&page_size=5&search=spring&status=published,draft&start_date=2026-05-01&end_date=2026-05-31&date_range=custom&sort_by=title&sort_order=asc", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotListFilter.Page != 2 || service.gotListFilter.PageSize != 5 {
		t.Fatalf("unexpected pagination filter: %#v", service.gotListFilter)
	}
	if service.gotListFilter.SearchTerm != "spring" {
		t.Fatalf("expected search term to be forwarded, got %q", service.gotListFilter.SearchTerm)
	}
	if len(service.gotListFilter.Statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %#v", service.gotListFilter.Statuses)
	}

	var payload EventListResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != 9 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestListEventsEndpointDefaultsAndErrors(t *testing.T) {
	service := &fakeEventService{}
	router := setupEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if service.gotListFilter.Page != 0 || service.gotListFilter.PageSize != 0 {
		t.Fatalf("expected controller to pass raw pagination values for service defaults, got %#v", service.gotListFilter)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/events?start_date=bad-date", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid date, got %d", res.Code)
	}
	assertEventAPIError(t, res, http.StatusBadRequest, "validation_error", "invalid date format; use RFC3339 or YYYY-MM-DD")

	router = setupEventRouter(&fakeEventService{listErr: ErrStoreUnavailable})
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/events", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 for service error, got %d", res.Code)
	}
	assertEventAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Event service is temporarily unavailable")
}

func TestCreateEventEndpointRejectsInvalidPayload(t *testing.T) {
	router := setupEventRouter(&fakeEventService{})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(`{"title":`))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
}

func TestCreateEventEndpointHandlesServiceUnavailable(t *testing.T) {
	router := setupEventRouter(&fakeEventService{createErr: ErrStoreUnavailable})

	body := `{"title":"Spring Fair","categories":["Events"],"event_type":"single_day_all_day","start_at":"2026-05-01T10:00:00Z","teaser":"Welcome!"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Event service is temporarily unavailable")
}

func TestUpdateEventEndpoint(t *testing.T) {
	service := &fakeEventService{}
	router := setupProtectedEventRouter(service)

	body := `{"title":"Updated Fair","categories":["Events"],"event_type":"single_day_all_day","start_at":"2026-05-01T10:00:00Z","teaser":"Updated!"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/events/12", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotUpdateID != 12 {
		t.Fatalf("expected update id 12, got %d", service.gotUpdateID)
	}
}

func TestUpdateEventEndpointRejectsBadID(t *testing.T) {
	router := setupEventRouter(&fakeEventService{})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/events/bad", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}

	payload := assertEventAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
		t.Fatalf("expected id validation detail, got %#v", payload)
	}
}

func TestUpdateEventEndpointRejectsInvalidPayload(t *testing.T) {
	router := setupEventRouter(&fakeEventService{})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/events/12", strings.NewReader(`{"title":`))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
}

func TestUpdateEventEndpointReturnsNotFound(t *testing.T) {
	router := setupEventRouter(&fakeEventService{updateErr: ErrEventNotFound})

	body := `{"title":"Updated Fair","categories":["Events"],"event_type":"single_day_all_day","start_at":"2026-05-01T10:00:00Z","teaser":"Updated!"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/events/12", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusNotFound, "not_found", "event not found")
}

func TestDeleteEventEndpoint(t *testing.T) {
	service := &fakeEventService{}
	router := setupProtectedEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/44", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDeleteID != 44 {
		t.Fatalf("expected delete id 44, got %d", service.gotDeleteID)
	}
}

func TestDeleteEventEndpointReturnsNotFound(t *testing.T) {
	router := setupEventRouter(&fakeEventService{deleteErr: ErrEventNotFound})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/44", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusNotFound, "not_found", "event not found")
}

func TestDeleteEventDocumentEndpoint(t *testing.T) {
	service := &fakeEventService{}
	router := setupProtectedEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/5/documents/9", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDeleteDocumentID != 5 || service.gotDeleteMediaID != 9 {
		t.Fatalf("unexpected ids: event=%d media=%d", service.gotDeleteDocumentID, service.gotDeleteMediaID)
	}
}

func TestDeleteEventDocumentEndpointRejectsBadMediaID(t *testing.T) {
	router := setupEventRouter(&fakeEventService{})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/5/documents/bad", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}

	payload := assertEventAPIError(t, res, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "mediaId" {
		t.Fatalf("expected mediaId validation detail, got %#v", payload)
	}
}

func TestDeleteAllEventDocumentsEndpoint(t *testing.T) {
	service := &fakeEventService{deleteAllResp: &DeleteAllDocumentsResponse{DeletedCount: 3}}
	router := setupProtectedEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/5/documents", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["deletedCount"] != float64(3) {
		t.Fatalf("expected deletedCount 3, got %v", payload["deletedCount"])
	}
}

func TestDeleteAllEventDocumentsEndpointHandlesError(t *testing.T) {
	router := setupEventRouter(&fakeEventService{deleteAllErr: ErrStoreUnavailable})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/5/documents", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusServiceUnavailable, "service_unavailable", "Event service is temporarily unavailable")
}

func TestDeleteEventPhotoEndpoint(t *testing.T) {
	service := &fakeEventService{}
	router := setupProtectedEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/5/photos/8", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if service.gotDeletePhotoID != 5 || service.gotDeletePhotoMedia != 8 {
		t.Fatalf("unexpected ids: event=%d media=%d", service.gotDeletePhotoID, service.gotDeletePhotoMedia)
	}
}

func TestDeleteEventPhotoEndpointHandlesError(t *testing.T) {
	router := setupEventRouter(&fakeEventService{deletePhotoErr: ErrEventMediaNotFound})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/events/5/photos/8", nil)
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", res.Code)
	}

	assertEventAPIError(t, res, http.StatusNotFound, "not_found", "event media not found")
}

func TestGetEventMediaContentEndpoint(t *testing.T) {
	service := &fakeEventService{
		mediaContentResp: &EventMediaContent{
			Content:     []byte("hello"),
			ContentType: "image/png",
			FileName:    "banner.png",
		},
	}
	router := setupEventRouter(service)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/5/media/8/content", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if res.Body.String() != "hello" {
		t.Fatalf("unexpected body: %q", res.Body.String())
	}
	if service.gotMediaContentID != 5 || service.gotMediaContentMediaID != 8 {
		t.Fatalf("unexpected media ids: event=%d media=%d", service.gotMediaContentID, service.gotMediaContentMediaID)
	}
	if got := res.Header().Get("Content-Disposition"); !strings.Contains(got, "banner.png") {
		t.Fatalf("expected content disposition filename, got %q", got)
	}
}

func TestGetEventMediaContentEndpointHandlesServiceError(t *testing.T) {
	router := setupEventRouter(&fakeEventService{mediaContentErr: ErrEventMediaNotFound})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/5/media/8/content", nil)
	router.ServeHTTP(res, req)

	assertEventAPIError(t, res, http.StatusNotFound, "not_found", "event media not found")
}

func TestProtectedWriteEndpointsRequireAuth(t *testing.T) {
	router := setupProtectedEventRouter(&fakeEventService{})

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/events", body: `{"title":"Spring Fair"}`},
		{method: http.MethodPut, path: "/api/events/12", body: `{"title":"Spring Fair"}`},
		{method: http.MethodDelete, path: "/api/events/12"},
		{method: http.MethodDelete, path: "/api/events/12/documents/4"},
		{method: http.MethodDelete, path: "/api/events/12/documents"},
		{method: http.MethodDelete, path: "/api/events/12/photos/7"},
	}

	for _, tc := range tests {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(res, req)
		assertEventAPIError(t, res, http.StatusUnauthorized, "missing_bearer_token", "Missing bearer token")
	}
}

func TestWriteEventErrorAndHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writeEventError(c, ErrMediaBucketNotConfigured)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
	assertEventAPIError(t, rec, http.StatusServiceUnavailable, "service_unavailable", "Event service is temporarily unavailable")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writeEventError(c, ErrEventMediaNotFound)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	assertEventAPIError(t, rec, http.StatusNotFound, "not_found", "event media not found")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	writeEventError(c, errors.New("bad request"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	assertEventAPIError(t, rec, http.StatusBadRequest, "validation_error", "bad request")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "10"}, {Key: "mediaId", Value: "11"}}
	eventID, mediaID, ok := pathEventAndMediaIDs(c)
	if !ok || eventID != 10 || mediaID != 11 {
		t.Fatalf("expected path ids 10 and 11, got %d and %d ok=%v", eventID, mediaID, ok)
	}

	dateVal, err := parseOptionalDate("2026-05-01")
	if err != nil || dateVal == nil {
		t.Fatalf("expected parseOptionalDate to succeed, got %v", err)
	}
	if got := parseQueryInt("bad"); got != -1 {
		t.Fatalf("expected parseQueryInt invalid marker -1, got %d", got)
	}
	values := splitQueryValues("published,draft", " published ")
	if len(values) != 3 {
		t.Fatalf("expected 3 split values, got %#v", values)
	}
}

func TestPathIntRejectsInvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "bad"}}

	if _, ok := pathInt(c, "id"); ok {
		t.Fatal("expected pathInt to reject invalid id")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	payload := assertEventAPIError(t, rec, http.StatusBadRequest, "validation_error", "Invalid path parameter")
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "id" {
		t.Fatalf("expected id validation detail, got %#v", payload)
	}
}

func TestRequestBindingParsesTimes(t *testing.T) {
	service := &fakeEventService{}
	router := setupProtectedEventRouter(service)

	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	body := `{"title":"Spring Fair","categories":["Events"],"event_type":"single_day_all_day","start_at":"` + now.Format(time.RFC3339) + `","teaser":"Welcome!"}`
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signEventTestToken(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if !service.gotCreateRequest.StartAt.Equal(now) {
		t.Fatalf("expected start_at %v, got %v", now, service.gotCreateRequest.StartAt)
	}
}

func assertEventAPIError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

func signEventTestToken(t *testing.T, secret string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": 7,
		"email":   "ada@example.com",
		"role":    "Admin",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return signed
}

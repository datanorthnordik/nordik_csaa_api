package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"nordikcsaaapi/internal/apiresponse"

	"github.com/gin-gonic/gin"
)

var errOne = errors.New("one")
var errTwo = errors.New("two")

func TestHandleErrorUsesMatchingRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/test", nil)

	HandleError(c, "test", errOne,
		ServiceUnavailableRule("temporarily unavailable", errTwo),
		ValidationRule(MatchAny(errOne)),
	)

	assertHTTPAPIError(t, rec, http.StatusBadRequest, "validation_error", "one")
}

func TestHandleErrorFallsBackToInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/test", nil)

	HandleError(c, "test", errors.New("boom"))

	assertHTTPAPIError(t, rec, http.StatusInternalServerError, "internal_error", "Internal server error")
}

func TestConflictAndNotFoundRules(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/test", nil)
	HandleError(c, "test", errors.New(`ERROR: duplicate key value violates unique constraint "x" (SQLSTATE 23505)`),
		ConflictRule("record already exists"),
	)
	assertHTTPAPIError(t, rec, http.StatusConflict, "conflict", "record already exists")

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/test/1", nil)
	HandleError(c, "test", errTwo, NotFoundRule(errTwo))
	assertHTTPAPIError(t, rec, http.StatusNotFound, "not_found", "two")
}

func TestMatchConflict(t *testing.T) {
	if !MatchConflict(errors.New(`ERROR: duplicate key value violates unique constraint "x" (SQLSTATE 23505)`)) {
		t.Fatal("expected duplicate key error to match conflict")
	}
	if MatchConflict(errors.New("plain validation error")) {
		t.Fatal("did not expect plain validation error to match conflict")
	}
}

func assertHTTPAPIError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string, message string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, rec.Code, rec.Body.String())
	}

	var payload apiresponse.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != code || payload.Error.Message != message {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

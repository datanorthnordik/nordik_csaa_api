package books

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/apiresponse"

	"github.com/gin-gonic/gin"
)

type bookMultipartTestFile struct {
	Field    string
	Filename string
	Data     []byte
}

func newBookMultipartRequest(t *testing.T, method string, target string, payload string, files []bookMultipartTestFile) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if payload != "" {
		if err := writer.WriteField("payload", payload); err != nil {
			t.Fatalf("write payload field: %v", err)
		}
	}

	for _, file := range files {
		part, err := writer.CreateFormFile(file.Field, file.Filename)
		if err != nil {
			t.Fatalf("create multipart form file: %v", err)
		}
		if _, err := part.Write(file.Data); err != nil {
			t.Fatalf("write multipart file: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func wrapPayloadString(t *testing.T, raw string) string {
	t.Helper()

	body, err := json.Marshal(map[string]string{"payload": raw})
	if err != nil {
		t.Fatalf("marshal wrapped payload string: %v", err)
	}
	return string(body)
}

func wrapPayloadObject(t *testing.T, raw string) string {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload object: %v", err)
	}

	body, err := json.Marshal(map[string]any{"payload": payload})
	if err != nil {
		t.Fatalf("marshal wrapped payload object: %v", err)
	}
	return string(body)
}

func TestBindSaveBookSubmissionRequestJSONBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const submissionPayload = `{"target_section_id":7,"new_section_name":"","field_values":[{"field_id":1,"value":"Athul"},{"field_id":5,"value":"<li><strong>4 cups</strong> flour</li>"}]}`

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/books/public/3/submissions", strings.NewReader(`{"payload":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSaveBookSubmissionRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
		assertBookAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("direct payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/books/public/3/submissions", strings.NewReader(submissionPayload))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindSaveBookSubmissionRequest(c)
		if !ok || req.TargetSectionID == nil || *req.TargetSectionID != 7 {
			t.Fatalf("expected direct payload bind success, got ok=%v req=%#v", ok, req)
		}
		if len(req.FieldValues) != 2 || req.FieldValues[1].Value != "<li><strong>4 cups</strong> flour</li>" {
			t.Fatalf("expected field values to be preserved, got %#v", req.FieldValues)
		}
	})

	t.Run("wrapped payload string", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/books/public/3/submissions", strings.NewReader(wrapPayloadString(t, submissionPayload)))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindSaveBookSubmissionRequest(c)
		if !ok || req.TargetSectionID == nil || *req.TargetSectionID != 7 {
			t.Fatalf("expected wrapped string payload bind success, got ok=%v req=%#v", ok, req)
		}
		if len(req.FieldValues) != 2 || req.FieldValues[0].Value != "Athul" {
			t.Fatalf("expected wrapped string payload values, got %#v", req.FieldValues)
		}
	})

	t.Run("wrapped payload object", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/books/public/3/submissions", strings.NewReader(wrapPayloadObject(t, submissionPayload)))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindSaveBookSubmissionRequest(c)
		if !ok || req.TargetSectionID == nil || *req.TargetSectionID != 7 {
			t.Fatalf("expected wrapped object payload bind success, got ok=%v req=%#v", ok, req)
		}
		if len(req.FieldValues) != 2 || req.FieldValues[1].FieldID != 5 {
			t.Fatalf("expected wrapped object payload values, got %#v", req.FieldValues)
		}
	})
}

func TestBindSaveBookSubmissionRequestMultipartBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const submissionPayload = `{"target_section_id":7,"field_values":[{"field_id":1,"value":"Athul"}]}`

	t.Run("missing payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodPost, "/api/books/public/3/submissions", strings.NewReader("--bad"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
		c.Request = req

		if _, ok := bindSaveBookSubmissionRequest(c); ok {
			t.Fatal("expected missing multipart payload to fail")
		}
	})

	t.Run("success without image", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newBookMultipartRequest(t, http.MethodPost, "/api/books/public/3/submissions", submissionPayload, nil)

		req, ok := bindSaveBookSubmissionRequest(c)
		if !ok || req.TargetSectionID == nil || *req.TargetSectionID != 7 {
			t.Fatalf("expected multipart bind success without image, got ok=%v req=%#v", ok, req)
		}
		if req.Image != nil {
			t.Fatalf("expected no image to be attached, got %#v", req.Image)
		}
	})
}

func TestBindUpdateBookSubmissionRequestWrappedJSONPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const submissionPayload = `{"target_section_id":7,"field_values":[{"field_id":1,"value":"Athul"}],"remove_image":true}`

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/books/3/submissions/8", strings.NewReader(wrapPayloadString(t, submissionPayload)))
	c.Request.Header.Set("Content-Type", "application/json")

	req, ok := bindUpdateBookSubmissionRequest(c)
	if !ok || req.TargetSectionID == nil || *req.TargetSectionID != 7 || !req.RemoveImage {
		t.Fatalf("expected wrapped update payload bind success, got ok=%v req=%#v", ok, req)
	}
}

func TestBindSaveBookVersionRequestWrappedJSONPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const versionPayload = `{"source_page_count":5,"content_template_page_number":1,"section_template_page_number":2,"allow_page_image":true,"allow_new_sections":true,"sections":[{"name":"Recipes","source_start_page":1,"source_end_page":5}],"fields":[{"label":"Name","input_type":"single_line","placement":"heading","show_label":true,"is_required":true,"is_email_field":false}]}`

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/books/3/versions", strings.NewReader(wrapPayloadObject(t, versionPayload)))
	c.Request.Header.Set("Content-Type", "application/json")

	req, ok := bindSaveBookVersionRequest(c)
	if !ok || req.SourcePageCount != 5 || len(req.Sections) != 1 || len(req.Fields) != 1 {
		t.Fatalf("expected wrapped version payload bind success, got ok=%v req=%#v", ok, req)
	}
}

func TestBindSaveBookVersionRequestIgnoresLayoutSettingsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const versionPayload = `{"source_page_count":5,"content_template_page_number":1,"section_template_page_number":2,"allow_page_image":true,"allow_new_sections":true,"layout_settings":{"heading_area":{"font_size":99}},"sections":[{"name":"Recipes","source_start_page":1,"source_end_page":5}],"fields":[{"label":"Name","input_type":"single_line","placement":"heading","show_label":true,"is_required":true,"is_email_field":false}]}`

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/books/3/versions", strings.NewReader(versionPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	req, ok := bindSaveBookVersionRequest(c)
	if !ok {
		t.Fatal("expected version payload with layout_settings to bind successfully")
	}
	if len(req.LayoutSettings) != 0 {
		t.Fatalf("expected layout_settings payload to be ignored, got %s", string(req.LayoutSettings))
	}
}

func assertBookAPIError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
	t.Helper()

	res := rec.Result()
	if res.StatusCode != wantStatus {
		t.Fatalf("expected status %d, got %d", wantStatus, res.StatusCode)
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

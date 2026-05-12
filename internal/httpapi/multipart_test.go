package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMultipartHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("payload", `{"name":"Homepage"}`); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	part, err := writer.CreateFormFile("cover_image_file", "cover.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("part.Write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/test", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	if !IsMultipartForm(c) {
		t.Fatal("expected multipart request to be detected")
	}

	payload, err := MultipartPayload(c, "payload")
	if err != nil {
		t.Fatalf("MultipartPayload returned error: %v", err)
	}
	if payload != `{"name":"Homepage"}` {
		t.Fatalf("unexpected payload: %q", payload)
	}

	file, err := ReadMultipartFile(c, "cover_image_file")
	if err != nil {
		t.Fatalf("ReadMultipartFile returned error: %v", err)
	}
	if file == nil {
		t.Fatal("expected uploaded file")
	}
	if file.Filename != "cover.png" {
		t.Fatalf("unexpected filename: %q", file.Filename)
	}
	if string(file.Data) != "hello" {
		t.Fatalf("unexpected file data: %q", string(file.Data))
	}
	if file.ContentType == "" {
		t.Fatal("expected content type to be detected")
	}

	missing, err := ReadMultipartFile(c, "missing")
	if err != nil {
		t.Fatalf("expected missing file to return nil, got error: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing file to return nil, got %#v", missing)
	}
}

func TestMultipartPayloadRequiresField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/test", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	if _, err := MultipartPayload(c, "payload"); err == nil {
		t.Fatal("expected missing payload error")
	}
}

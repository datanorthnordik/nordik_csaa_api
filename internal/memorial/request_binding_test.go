package memorial

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type memorialMultipartTestFile struct {
	Field       string
	Filename    string
	ContentType string
	Data        []byte
}

func newMemorialMultipartRequest(t *testing.T, method string, target string, payload string, files []memorialMultipartTestFile) *http.Request {
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

func TestBindSaveMemorialRequestJSONBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/memorial", strings.NewReader(`{"full_name":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSaveMemorialRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
		assertMemorialAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("embedded data url", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/memorial", strings.NewReader(`{"full_name":"Ada Lovelace","category":"founder","status":"draft","portrait":{"file_url":"data:image/png;base64,aGVsbG8="}}`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSaveMemorialRequest(c); ok {
			t.Fatal("expected embedded data url bind to fail")
		}
		assertMemorialAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/memorial", strings.NewReader(`{"full_name":"Ada Lovelace","category":"founder","status":"draft","biography":"<p>Hello</p>"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindSaveMemorialRequest(c)
		if !ok || req.FullName != "Ada Lovelace" || req.Category != "founder" {
			t.Fatalf("expected bind success, got ok=%v req=%#v", ok, req)
		}
	})
}

func TestBindSaveMemorialRequestMultipartBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodPost, "/api/memorial", strings.NewReader("--bad"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
		c.Request = req

		if _, ok := bindSaveMemorialRequest(c); ok {
			t.Fatal("expected missing multipart payload to fail")
		}
	})

	t.Run("invalid payload json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newMemorialMultipartRequest(t, http.MethodPost, "/api/memorial", `{"full_name":`, nil)

		if _, ok := bindSaveMemorialRequest(c); ok {
			t.Fatal("expected invalid multipart payload json to fail")
		}
		assertMemorialAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newMemorialMultipartRequest(t, http.MethodPost, "/api/memorial", `{"full_name":"Ada Lovelace","category":"founder","status":"draft","portrait":{"file_name":"Portrait"},"gallery_images":[{"file_name":"Gallery One"}]}`, []memorialMultipartTestFile{
			{Field: "portrait_file", Filename: "portrait.png", Data: []byte("portrait")},
			{Field: memorialGalleryImageFileField(0), Filename: "gallery.txt", Data: []byte("gallery")},
		})

		req, ok := bindSaveMemorialRequest(c)
		if !ok || req.Portrait == nil || len(req.GalleryImages) != 1 {
			t.Fatalf("expected multipart bind success, got ok=%v req=%#v", ok, req)
		}
		if string(req.Portrait.Content) != "portrait" || string(req.GalleryImages[0].Content) != "gallery" {
			t.Fatalf("expected uploaded bytes to be attached, got %#v", req)
		}
		if !strings.HasPrefix(req.GalleryImages[0].MimeType, "text/plain") {
			t.Fatalf("expected detected mime type, got %#v", req.GalleryImages[0])
		}
	})

	t.Run("embedded data url in multipart payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newMemorialMultipartRequest(t, http.MethodPost, "/api/memorial", `{"full_name":"Ada Lovelace","category":"founder","status":"draft","gallery_images":[{"file_url":"data:image/png;base64,aGVsbG8="}]}`, nil)

		if _, ok := bindSaveMemorialRequest(c); ok {
			t.Fatal("expected multipart embedded data url bind to fail")
		}
		assertMemorialAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})
}

func TestMemorialBindingHelpersAdditionalBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	input := &MemorialUploadInput{
		FileName: "keep.jpg",
		MimeType: "image/custom",
		FileURL:  " gs://drive-bucket/memorial/keep.jpg ",
	}
	applyUploadedFile(input, &httpapi.UploadedFile{
		Filename:    "portrait.jpg",
		ContentType: "image/jpeg",
		Data:        []byte("portrait"),
	})
	if input.FileName != "keep.jpg" || input.MimeType != "image/custom" {
		t.Fatalf("expected existing metadata to be preserved, got %#v", input)
	}
	if input.FileSize != int64(len("portrait")) || string(input.Content) != "portrait" {
		t.Fatalf("expected helper to populate upload fields, got %#v", input)
	}
	if input.FileURL != "gs://drive-bucket/memorial/keep.jpg" {
		t.Fatalf("expected file url to be trimmed, got %#v", input)
	}

	applyUploadedFile(nil, &httpapi.UploadedFile{Data: []byte("ignored")})
	applyUploadedFile(&MemorialUploadInput{}, nil)

	var ptrInput *MemorialUploadInput
	applyUploadedFilePtr(&ptrInput, &httpapi.UploadedFile{
		Filename:    "portrait.jpg",
		ContentType: "application/octet-stream",
		Data:        []byte("portrait"),
	})
	if ptrInput == nil || ptrInput.FileName != "portrait.jpg" || string(ptrInput.Content) != "portrait" {
		t.Fatalf("expected pointer helper to create and populate input, got %#v", ptrInput)
	}

	existing := &MemorialUploadInput{FileName: "existing.jpg"}
	applyUploadedFilePtr(&existing, nil)
	if existing.FileName != "existing.jpg" {
		t.Fatalf("expected nil pointer helper call to leave input unchanged, got %#v", existing)
	}

	if got := detectUploadedContentType("image/jpeg", []byte("portrait")); got != "image/jpeg" {
		t.Fatalf("expected explicit content type to be preserved, got %q", got)
	}
	if got := detectUploadedContentType("application/octet-stream", []byte("portrait")); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("expected detected content type for octet-stream, got %q", got)
	}
	if memorialRequestUsesEmbeddedBase64(SaveMemorialRequest{
		Portrait: &MemorialUploadInput{FileURL: "https://example.com/portrait.jpg"},
	}) {
		t.Fatal("expected regular URLs to be accepted")
	}
}

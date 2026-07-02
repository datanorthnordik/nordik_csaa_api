package bookshelf

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type bookshelfMultipartTestFile struct {
	Field    string
	Filename string
	Data     []byte
}

func newBookshelfMultipartRequest(t *testing.T, method string, target string, payload string, files []bookshelfMultipartTestFile) *http.Request {
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

func TestBindSaveBookshelfEntryRequestJSONBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/bookshelf", strings.NewReader(`{"author":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSaveBookshelfEntryRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
		assertBookshelfBindError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/bookshelf", strings.NewReader(`{"author":"Author","title":"Book","description":"Desc","remove_author_image":true,"remove_cover_image":true}`))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindSaveBookshelfEntryRequest(c)
		if !ok || req.Author != "Author" || req.Title != "Book" || !req.RemoveAuthorImage || !req.RemoveCoverImage {
			t.Fatalf("expected bind success, got ok=%v req=%#v", ok, req)
		}
	})

	t.Run("embedded data url", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/bookshelf", strings.NewReader(`{"author":"Author","title":"Book","description":"Desc","book_upload":{"file_url":"data:application/pdf;base64,aGVsbG8="}}`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSaveBookshelfEntryRequest(c); ok {
			t.Fatal("expected embedded data url bind to fail")
		}
		assertBookshelfBindError(t, rec, http.StatusBadRequest, "validation_error", multipartBookshelfUploadValidationMessage)
	})
}

func TestBindSaveBookshelfEntryRequestMultipartBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const payload = `{"author":"Author","title":"Book","description":"Desc"}`

	t.Run("missing payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodPost, "/api/bookshelf", strings.NewReader("--bad"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
		c.Request = req

		if _, ok := bindSaveBookshelfEntryRequest(c); ok {
			t.Fatal("expected missing multipart payload to fail")
		}
	})

	t.Run("success with book author image and cover", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newBookshelfMultipartRequest(t, http.MethodPost, "/api/bookshelf", payload, []bookshelfMultipartTestFile{
			{Field: "book_file", Filename: "book.pdf", Data: []byte("%PDF-1.4")},
			{Field: "author_image_file", Filename: "author.png", Data: []byte("author-bytes")},
			{Field: "cover_image_file", Filename: "cover.png", Data: []byte("png-bytes")},
		})

		req, ok := bindSaveBookshelfEntryRequest(c)
		if !ok {
			t.Fatal("expected multipart bind success")
		}
		if req.BookUpload == nil || req.BookUpload.FileName != "book.pdf" || string(req.BookUpload.Content) != "%PDF-1.4" {
			t.Fatalf("expected book upload to be attached, got %#v", req.BookUpload)
		}
		if req.AuthorImage == nil || req.AuthorImage.FileName != "author.png" || string(req.AuthorImage.Content) != "author-bytes" {
			t.Fatalf("expected author image upload to be attached, got %#v", req.AuthorImage)
		}
		if req.CoverImage == nil || req.CoverImage.FileName != "cover.png" || string(req.CoverImage.Content) != "png-bytes" {
			t.Fatalf("expected cover upload to be attached, got %#v", req.CoverImage)
		}
	})
}

func TestBookshelfRequestBindingHelpers(t *testing.T) {
	if _, _, err := readOptionalBookshelfFile(nil, "book_file"); err == nil {
		t.Fatal("expected nil context to fail")
	}

	input := &BookshelfUploadInput{
		FileName: "keep.pdf",
		MimeType: "application/custom",
		FileURL:  " gs://drive-bucket/path/keep.pdf ",
	}
	applyBookshelfUploadedFile(input, nil)
	applyBookshelfUploadedFile(nil, nil)
	applyBookshelfUploadedFile(input, &httpapi.UploadedFile{
		Filename:    "book.pdf",
		ContentType: "application/pdf",
		Data:        []byte("book"),
	})
	if input.FileName != "keep.pdf" || input.MimeType != "application/custom" || input.FileURL != "gs://drive-bucket/path/keep.pdf" {
		t.Fatalf("expected existing file metadata to be preserved, got %#v", input)
	}
	if input.FileSize != int64(len("book")) || string(input.Content) != "book" {
		t.Fatalf("expected upload content to be populated, got %#v", input)
	}
	if got := detectBookshelfUploadedContentType("application/pdf", []byte("book")); got != "application/pdf" {
		t.Fatalf("expected explicit content type to be preserved, got %q", got)
	}
}

func assertBookshelfBindError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string) apiresponse.ErrorResponse {
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

package newsletters

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

func TestBindSaveNewsletterEntryRequestJSONBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/newsletters", strings.NewReader(`{"title":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSaveNewsletterEntryRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
		assertNewsletterAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/newsletters", strings.NewReader(`{"title":"Spring Update","category":"csaa","send_date":"2026-05-01","status":"draft","visibility":"private"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindSaveNewsletterEntryRequest(c)
		if !ok || req.Title != "Spring Update" || req.Category != "csaa" {
			t.Fatalf("expected bind success, got ok=%v req=%#v", ok, req)
		}
	})
}

func TestBindAddNewsletterMediaRequestBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/newsletters/4/media", strings.NewReader(`{"media":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindAddNewsletterMediaRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
		assertNewsletterAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("embedded data url", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/newsletters/4/media", strings.NewReader(`{"media":[{"file_url":"data:application/pdf;base64,aGVsbG8="}]}`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindAddNewsletterMediaRequest(c); ok {
			t.Fatal("expected embedded media data url bind to fail")
		}
		assertNewsletterAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})

	t.Run("multipart generic files", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newNewsletterMultipartRequest(t, http.MethodPost, "/api/newsletters/4/media", `{"media":[{"display_name":"Agenda"}]}`, []multipartUploadTestFile{
			{Field: "files[]", Filename: "agenda.pdf", Data: []byte("agenda")},
			{Field: "files[]", Filename: "minutes.pdf", Data: []byte("minutes")},
		})

		req, ok := bindAddNewsletterMediaRequest(c)
		if !ok || len(req.Media) != 2 {
			t.Fatalf("expected multipart generic files bind success, got ok=%v req=%#v", ok, req)
		}
		if string(req.Media[0].Content) != "agenda" || string(req.Media[1].Content) != "minutes" {
			t.Fatalf("expected media content bytes, got %#v", req.Media)
		}
	})

	t.Run("multipart indexed files", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newNewsletterMultipartRequest(t, http.MethodPost, "/api/newsletters/4/media", `{"media":[{"display_name":"Agenda"},{"display_name":"Minutes"}]}`, []multipartUploadTestFile{
			{Field: "media[0].file", Filename: "agenda.pdf", Data: []byte("agenda")},
			{Field: "media[1].file", Filename: "minutes.pdf", Data: []byte("minutes")},
		})

		req, ok := bindAddNewsletterMediaRequest(c)
		if !ok || len(req.Media) != 2 {
			t.Fatalf("expected indexed multipart bind success, got ok=%v req=%#v", ok, req)
		}
		if string(req.Media[0].Content) != "agenda" || string(req.Media[1].Content) != "minutes" {
			t.Fatalf("expected indexed media content bytes, got %#v", req.Media)
		}
	})

	t.Run("multipart invalid payload json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newNewsletterMultipartRequest(t, http.MethodPost, "/api/newsletters/4/media", `{"media":`, nil)

		if _, ok := bindAddNewsletterMediaRequest(c); ok {
			t.Fatal("expected invalid multipart payload json bind to fail")
		}
		assertNewsletterAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("multipart embedded data url", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newNewsletterMultipartRequest(t, http.MethodPost, "/api/newsletters/4/media", `{"media":[{"file_url":"data:application/pdf;base64,aGVsbG8="}]}`, nil)

		if _, ok := bindAddNewsletterMediaRequest(c); ok {
			t.Fatal("expected multipart embedded media data url bind to fail")
		}
		assertNewsletterAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})

	t.Run("multipart octet stream detection", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newNewsletterMultipartRequest(t, http.MethodPost, "/api/newsletters/4/media", `{"media":[{"display_name":"Agenda"}]}`, []multipartUploadTestFile{
			{Field: "media[0].file", Filename: "agenda.txt", ContentType: "application/octet-stream", Data: []byte("agenda")},
		})

		req, ok := bindAddNewsletterMediaRequest(c)
		if !ok || len(req.Media) != 1 {
			t.Fatalf("expected octet-stream multipart bind success, got ok=%v req=%#v", ok, req)
		}
		if !strings.HasPrefix(req.Media[0].MimeType, "text/plain") {
			t.Fatalf("expected detected text/plain mime type, got %#v", req.Media[0])
		}
	})

	t.Run("multipart missing payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodPost, "/api/newsletters/4/media", strings.NewReader("--bad"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
		c.Request = req

		if _, ok := bindAddNewsletterMediaRequest(c); ok {
			t.Fatal("expected missing multipart payload bind to fail")
		}
	})
}

func TestBindOtherNewsletterRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("update media invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/newsletters/4/media/1", strings.NewReader(`{"display_name":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindUpdateNewsletterMediaRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
	})

	t.Run("delete media invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/newsletters/4/media", strings.NewReader(`{"media_ids":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindDeleteNewsletterMediaRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
	})

	t.Run("reorder media invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/newsletters/4/media/order", strings.NewReader(`{"media_ids":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindReorderNewsletterMediaRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/newsletters/4/media/1", strings.NewReader(`{"display_name":"Agenda","file_name":"agenda.pdf"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindUpdateNewsletterMediaRequest(c)
		if !ok || req.DisplayName != "Agenda" {
			t.Fatalf("expected update bind success, got ok=%v req=%#v", ok, req)
		}

		rec = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/newsletters/4/media", strings.NewReader(`{"media_ids":[1,2]}`))
		c.Request.Header.Set("Content-Type", "application/json")
		deleteReq, ok := bindDeleteNewsletterMediaRequest(c)
		if !ok || len(deleteReq.MediaIDs) != 2 {
			t.Fatalf("expected delete bind success, got ok=%v req=%#v", ok, deleteReq)
		}

		rec = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/newsletters/4/media/order", strings.NewReader(`{"media_ids":[2,1]}`))
		c.Request.Header.Set("Content-Type", "application/json")
		reorderReq, ok := bindReorderNewsletterMediaRequest(c)
		if !ok || len(reorderReq.MediaIDs) != 2 {
			t.Fatalf("expected reorder bind success, got ok=%v req=%#v", ok, reorderReq)
		}
	})
}

func TestBindRequestHelpersAdditionalBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	if _, err := readOptionalMultipartFile(nil, "media[0].file"); err == nil {
		t.Fatal("expected readOptionalMultipartFile to fail for nil context")
	}

	input := &NewsletterUploadInput{
		FileName: "keep.pdf",
		MimeType: "application/custom",
		FileURL:  " gs://drive-bucket/path/keep.pdf ",
	}
	applyNewsletterUploadedFile(input, &httpapi.UploadedFile{
		Filename:    "agenda.pdf",
		ContentType: "application/pdf",
		Data:        []byte("agenda"),
	})
	if input.FileName != "keep.pdf" || input.MimeType != "application/custom" {
		t.Fatalf("expected existing file metadata to be preserved, got %#v", input)
	}
	if input.DisplayName != "keep.pdf" || input.FileSize != int64(len("agenda")) || string(input.Content) != "agenda" {
		t.Fatalf("expected uploaded file helper to populate content fields, got %#v", input)
	}
	if input.FileURL != "gs://drive-bucket/path/keep.pdf" {
		t.Fatalf("expected file url to be trimmed, got %#v", input)
	}

	applyNewsletterUploadedFile(nil, &httpapi.UploadedFile{Data: []byte("ignored")})
	applyNewsletterUploadedFile(&NewsletterUploadInput{}, nil)

	if got := detectUploadedContentType("application/pdf", []byte("agenda")); got != "application/pdf" {
		t.Fatalf("expected explicit content type to be preserved, got %q", got)
	}
	if isEmbeddedDataURL("https://example.com/file.pdf") {
		t.Fatal("expected non-data url to be rejected")
	}
}

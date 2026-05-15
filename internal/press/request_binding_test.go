package press

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

func TestBindSavePressEntryRequestJSONBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader(`{"title":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSavePressEntryRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("embedded data url", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader(`{"title":"Spring Fair","release_date":"2026-05-01","status":"draft","visibility":"private","cover_image":{"file_url":"data:image/png;base64,aGVsbG8="}}`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindSavePressEntryRequest(c); ok {
			t.Fatal("expected embedded data url bind to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader(`{"title":"Spring Fair","release_date":"2026-05-01","status":"draft","visibility":"private"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		req, ok := bindSavePressEntryRequest(c)
		if !ok || req.Title != "Spring Fair" {
			t.Fatalf("expected bind success, got ok=%v req=%#v", ok, req)
		}
	})
}

func TestBindSavePressEntryRequestMultipartBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader("--bad"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
		c.Request = req

		if _, ok := bindSavePressEntryRequest(c); ok {
			t.Fatal("expected missing multipart payload to fail")
		}
	})

	t.Run("invalid payload json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press", `{"title":`, nil)

		if _, ok := bindSavePressEntryRequest(c); ok {
			t.Fatal("expected invalid multipart payload json to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press", `{"title":"Spring Fair","release_date":"2026-05-01","status":"draft","visibility":"private","cover_image":{"display_name":"Poster"}}`, []multipartUploadTestFile{
			{Field: "cover_image_file", Filename: "poster.png", Data: []byte("poster")},
		})

		req, ok := bindSavePressEntryRequest(c)
		if !ok || req.CoverImage == nil || string(req.CoverImage.Content) != "poster" {
			t.Fatalf("expected multipart bind success, got ok=%v req=%#v", ok, req)
		}
	})

	t.Run("embedded data url in multipart payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press", `{"title":"Spring Fair","release_date":"2026-05-01","status":"draft","visibility":"private","cover_image":{"file_url":"data:image/png;base64,aGVsbG8="}}`, nil)

		if _, ok := bindSavePressEntryRequest(c); ok {
			t.Fatal("expected multipart embedded data url bind to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})
}

func TestBindAddPressMediaRequestBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/press/4/media", strings.NewReader(`{"media":`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindAddPressMediaRequest(c); ok {
			t.Fatal("expected invalid json bind to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("embedded data url", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/press/4/media", strings.NewReader(`{"media":[{"file_url":"data:application/pdf;base64,aGVsbG8="}]}`))
		c.Request.Header.Set("Content-Type", "application/json")

		if _, ok := bindAddPressMediaRequest(c); ok {
			t.Fatal("expected embedded media data url bind to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})

	t.Run("multipart generic files", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press/4/media", `{"media":[{"display_name":"Agenda"}]}`, []multipartUploadTestFile{
			{Field: "files[]", Filename: "agenda.pdf", Data: []byte("agenda")},
			{Field: "files[]", Filename: "minutes.pdf", Data: []byte("minutes")},
		})

		req, ok := bindAddPressMediaRequest(c)
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
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press/4/media", `{"media":[{"display_name":"Agenda"},{"display_name":"Minutes"}]}`, []multipartUploadTestFile{
			{Field: "media[0].file", Filename: "agenda.pdf", Data: []byte("agenda")},
			{Field: "media[1].file", Filename: "minutes.pdf", Data: []byte("minutes")},
		})

		req, ok := bindAddPressMediaRequest(c)
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
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press/4/media", `{"media":`, nil)

		if _, ok := bindAddPressMediaRequest(c); ok {
			t.Fatal("expected invalid multipart payload json bind to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	})

	t.Run("multipart embedded data url", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press/4/media", `{"media":[{"file_url":"data:application/pdf;base64,aGVsbG8="}]}`, nil)

		if _, ok := bindAddPressMediaRequest(c); ok {
			t.Fatal("expected multipart embedded media data url bind to fail")
		}
		assertPressAPIError(t, rec, http.StatusBadRequest, "validation_error", multipartUploadValidationMessage)
	})

	t.Run("multipart octet stream detection", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = newPressMultipartRequest(t, http.MethodPost, "/api/press/4/media", `{"media":[{"display_name":"Agenda"}]}`, []multipartUploadTestFile{
			{Field: "media[0].file", Filename: "agenda.txt", ContentType: "application/octet-stream", Data: []byte("agenda")},
		})

		req, ok := bindAddPressMediaRequest(c)
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
		req := httptest.NewRequest(http.MethodPost, "/api/press/4/media", strings.NewReader("--bad"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
		c.Request = req

		if _, ok := bindAddPressMediaRequest(c); ok {
			t.Fatal("expected missing multipart payload bind to fail")
		}
	})
}

func TestBindRequestHelpersAdditionalBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	if _, err := readOptionalMultipartFile(nil, "cover_image_file"); err == nil {
		t.Fatal("expected readOptionalMultipartFile to fail for nil context")
	}

	input := &PressUploadInput{
		FileName: "keep.pdf",
		MimeType: "application/custom",
		FileURL:  " gs://drive-bucket/path/keep.pdf ",
	}
	applyPressUploadedFile(input, &httpapi.UploadedFile{
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

	applyPressUploadedFile(nil, &httpapi.UploadedFile{Data: []byte("ignored")})
	applyPressUploadedFile(&PressUploadInput{}, nil)

	ptrInput := &PressUploadInput{DisplayName: "Existing"}
	applyPressUploadedFilePtr(&ptrInput, nil)
	if ptrInput.DisplayName != "Existing" {
		t.Fatalf("expected nil pointer helper call to leave input unchanged, got %#v", ptrInput)
	}

	if got := detectUploadedContentType("application/pdf", []byte("agenda")); got != "application/pdf" {
		t.Fatalf("expected explicit content type to be preserved, got %q", got)
	}
	if isEmbeddedDataURL("https://example.com/file.pdf") {
		t.Fatal("expected non-data url to be rejected")
	}
}

func TestBindMutationRequestHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		body     string
		bindFunc func(*gin.Context) bool
	}{
		{
			name: "update media invalid json",
			body: `{"display_name":`,
			bindFunc: func(c *gin.Context) bool {
				_, ok := bindUpdatePressMediaRequest(c)
				return ok
			},
		},
		{
			name: "delete media invalid json",
			body: `{"media_ids":`,
			bindFunc: func(c *gin.Context) bool {
				_, ok := bindDeletePressMediaRequest(c)
				return ok
			},
		},
		{
			name: "reorder media invalid json",
			body: `{"media_ids":`,
			bindFunc: func(c *gin.Context) bool {
				_, ok := bindReorderPressMediaRequest(c)
				return ok
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/press", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")

			if tc.bindFunc(c) {
				t.Fatal("expected helper bind to fail")
			}
			assertPressAPIError(t, rec, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
		})
	}
}

func TestReadMultipartFilesAndApplyHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("non multipart request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/press", nil)

		files, err := readMultipartFiles(c, "files")
		if err != nil || files != nil {
			t.Fatalf("expected nil files without error, got files=%#v err=%v", files, err)
		}
	})

	t.Run("helper pointer apply", func(t *testing.T) {
		var input *PressUploadInput
		applyPressUploadedFilePtr(&input, &httpapi.UploadedFile{
			Filename:    "agenda.pdf",
			ContentType: "application/pdf",
			Data:        []byte("agenda"),
		})
		if input == nil || input.FileName != "agenda.pdf" || string(input.Content) != "agenda" {
			t.Fatalf("expected helper to create and populate input, got %#v", input)
		}
	})
}

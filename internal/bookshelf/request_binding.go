package bookshelf

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartBookshelfUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveBookshelfEntryRequest(c *gin.Context) (SaveBookshelfEntryRequest, bool) {
	var req SaveBookshelfEntryRequest

	if httpapi.IsMultipartForm(c) {
		payload, err := httpapi.MultipartPayload(c, "payload")
		if err != nil {
			apiresponse.WriteValidationError(c, err.Error())
			return req, false
		}

		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			apiresponse.WriteBindingError(c, err, req)
			return req, false
		}

		if bookshelfRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartBookshelfUploadValidationMessage)
			return req, false
		}

		bookFile, hasBookFile, err := readOptionalBookshelfFile(c, "book_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid book upload")
			return req, false
		}
		if hasBookFile {
			applyBookshelfUploadedFilePtr(&req.BookUpload, bookFile)
		}

		authorImageFile, hasAuthorImageFile, err := readOptionalBookshelfFile(c, "author_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid author image upload")
			return req, false
		}
		if hasAuthorImageFile {
			applyBookshelfUploadedFilePtr(&req.AuthorImage, authorImageFile)
		}

		coverFile, hasCoverFile, err := readOptionalBookshelfFile(c, "cover_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid cover image upload")
			return req, false
		}
		if hasCoverFile {
			applyBookshelfUploadedFilePtr(&req.CoverImage, coverFile)
		}

		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	if bookshelfRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartBookshelfUploadValidationMessage)
		return req, false
	}

	return req, true
}

func readOptionalBookshelfFile(c *gin.Context, field string) (*httpapi.UploadedFile, bool, error) {
	file, err := httpapi.ReadMultipartFile(c, field)
	if err == nil {
		return file, file != nil, nil
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if errors.Is(err, http.ErrMissingFile) ||
		strings.Contains(message, "no such file") ||
		strings.Contains(message, "missing file") ||
		strings.Contains(message, "request content-type isn't multipart/form-data") {
		return nil, false, nil
	}

	return nil, false, err
}

func applyBookshelfUploadedFile(input *BookshelfUploadInput, file *httpapi.UploadedFile) {
	if input == nil || file == nil {
		return
	}

	input.Content = append([]byte(nil), file.Data...)
	if strings.TrimSpace(input.FileName) == "" {
		input.FileName = strings.TrimSpace(file.Filename)
	}
	if strings.TrimSpace(input.MimeType) == "" {
		input.MimeType = detectBookshelfUploadedContentType(file.ContentType, file.Data)
	}
	input.FileSize = int64(len(file.Data))
	input.FileURL = strings.TrimSpace(input.FileURL)
	input.GCPObjectKey = strings.TrimSpace(input.GCPObjectKey)
}

func applyBookshelfUploadedFilePtr(input **BookshelfUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *input == nil {
		*input = &BookshelfUploadInput{}
	}
	applyBookshelfUploadedFile(*input, file)
}

func detectBookshelfUploadedContentType(contentType string, data []byte) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" || strings.EqualFold(contentType, "application/octet-stream") {
		return http.DetectContentType(data)
	}
	return contentType
}

func bookshelfRequestUsesEmbeddedBase64(req SaveBookshelfEntryRequest) bool {
	return bookshelfInputUsesEmbeddedBase64(req.BookUpload) ||
		bookshelfInputUsesEmbeddedBase64(req.AuthorImage) ||
		bookshelfInputUsesEmbeddedBase64(req.CoverImage)
}

func bookshelfInputUsesEmbeddedBase64(input *BookshelfUploadInput) bool {
	if input == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input.FileURL)), "data:")
}

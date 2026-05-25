package resources

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveResourceRequest(c *gin.Context) (SaveResourceRequest, bool) {
	var req SaveResourceRequest

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

		if resourceRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		file, hasFile, err := readOptionalResourceFile(c)
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid resource file upload")
			return req, false
		}

		if hasFile {
			applyUploadedFilePtr(&req.Document, file)
		}

		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	if resourceRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func readOptionalResourceFile(c *gin.Context) (*httpapi.UploadedFile, bool, error) {
	file, err := httpapi.ReadMultipartFile(c, "resource_file")
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

func applyUploadedFile(input *ResourceUploadInput, file *httpapi.UploadedFile) {
	if input == nil || file == nil {
		return
	}

	input.Content = append([]byte(nil), file.Data...)

	if strings.TrimSpace(input.FileName) == "" {
		input.FileName = strings.TrimSpace(file.Filename)
	}

	if strings.TrimSpace(input.MimeType) == "" {
		input.MimeType = detectUploadedContentType(file.ContentType, file.Data)
	}

	input.FileSize = int64(len(file.Data))
	input.FileURL = strings.TrimSpace(input.FileURL)
}

func applyUploadedFilePtr(input **ResourceUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}

	if *input == nil {
		*input = &ResourceUploadInput{}
	}

	applyUploadedFile(*input, file)
}

func detectUploadedContentType(contentType string, data []byte) string {
	contentType = strings.TrimSpace(contentType)

	if contentType == "" || strings.EqualFold(contentType, "application/octet-stream") {
		return http.DetectContentType(data)
	}

	return contentType
}

func resourceRequestUsesEmbeddedBase64(req SaveResourceRequest) bool {
	if req.Document == nil {
		return false
	}

	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(req.Document.FileURL)), "data:")
}

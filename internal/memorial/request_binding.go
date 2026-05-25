package memorial

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveMemorialRequest(c *gin.Context) (SaveMemorialRequest, bool) {
	var req SaveMemorialRequest

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
		if memorialRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		portraitFile, err := httpapi.ReadMultipartFile(c, "portrait_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid portrait upload")
			return req, false
		}
		applyUploadedFilePtr(&req.Portrait, portraitFile)

		for idx := range req.GalleryImages {
			file, err := httpapi.ReadMultipartFile(c, memorialGalleryImageFileField(idx))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid gallery image upload")
				return req, false
			}
			applyUploadedFile(&req.GalleryImages[idx], file)
		}

		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if memorialRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func memorialGalleryImageFileField(idx int) string {
	return fmt.Sprintf("gallery_images[%d].file", idx)
}

func applyUploadedFile(input *MemorialUploadInput, file *httpapi.UploadedFile) {
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
	input.GCPObjectKey = strings.TrimSpace(input.GCPObjectKey)
}

func applyUploadedFilePtr(input **MemorialUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *input == nil {
		*input = &MemorialUploadInput{}
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

func memorialRequestUsesEmbeddedBase64(req SaveMemorialRequest) bool {
	if req.Portrait != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(req.Portrait.FileURL)), "data:") {
		return true
	}
	for _, image := range req.GalleryImages {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(image.FileURL)), "data:") {
			return true
		}
	}
	return false
}

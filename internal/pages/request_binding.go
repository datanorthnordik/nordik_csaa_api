package pages

import (
	"encoding/json"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSavePageRequest(c *gin.Context) (SavePageRequest, bool) {
	var req SavePageRequest

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
		if pageRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		file, err := httpapi.ReadMultipartFile(c, "hero_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyPageUploadedFile(&req.HeroImage, file)
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if pageRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func pageRequestUsesEmbeddedBase64(req SavePageRequest) bool {
	return req.HeroImage != nil && strings.TrimSpace(req.HeroImage.DataBase64) != ""
}

func applyPageUploadedFile(dst **PageUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *dst == nil {
		*dst = &PageUploadInput{}
	}

	if strings.TrimSpace((*dst).FileName) == "" {
		(*dst).FileName = file.Filename
	}
	if strings.TrimSpace((*dst).MimeType) == "" {
		(*dst).MimeType = file.ContentType
	}
	(*dst).DataBase64 = ""
	(*dst).Content = append([]byte(nil), file.Data...)
}

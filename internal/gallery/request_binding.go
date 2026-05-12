package gallery

import (
	"encoding/json"
	"fmt"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveGalleryRequest(c *gin.Context) (SaveGalleryRequest, bool) {
	var req SaveGalleryRequest

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
		if saveGalleryRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		file, err := httpapi.ReadMultipartFile(c, "cover_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyGalleryUploadedFilePtr(&req.CoverImage, file)
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if saveGalleryRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func bindAddGalleryImagesRequest(c *gin.Context) (AddGalleryImagesRequest, bool) {
	var req AddGalleryImagesRequest

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
		if addGalleryImagesRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		for idx := range req.Images {
			file, err := httpapi.ReadMultipartFile(c, galleryImageFileField(idx))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid multipart form data")
				return req, false
			}
			applyGalleryUploadedFile(&req.Images[idx], file)
		}
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if addGalleryImagesRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func saveGalleryRequestUsesEmbeddedBase64(req SaveGalleryRequest) bool {
	return req.CoverImage != nil && strings.TrimSpace(req.CoverImage.DataBase64) != ""
}

func addGalleryImagesRequestUsesEmbeddedBase64(req AddGalleryImagesRequest) bool {
	for _, image := range req.Images {
		if strings.TrimSpace(image.DataBase64) != "" {
			return true
		}
	}
	return false
}

func applyGalleryUploadedFile(dst *GalleryUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}

	if strings.TrimSpace(dst.FileName) == "" {
		dst.FileName = file.Filename
	}
	if strings.TrimSpace(dst.MimeType) == "" {
		dst.MimeType = file.ContentType
	}
	dst.DataBase64 = ""
	dst.Content = append([]byte(nil), file.Data...)
}

func applyGalleryUploadedFilePtr(dst **GalleryUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *dst == nil {
		*dst = &GalleryUploadInput{}
	}
	applyGalleryUploadedFile(*dst, file)
}

func galleryImageFileField(idx int) string {
	return fmt.Sprintf("images[%d].file", idx)
}

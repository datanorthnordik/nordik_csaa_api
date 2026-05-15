package press

import (
	"encoding/json"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSavePressEntryRequest(c *gin.Context) (SavePressEntryRequest, bool) {
	var req SavePressEntryRequest

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

		file, err := httpapi.ReadMultipartFile(c, "cover_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyPressUploadedFilePtr(&req.CoverImage, file)
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func bindAddPressMediaRequest(c *gin.Context) (AddPressMediaRequest, bool) {
	var req AddPressMediaRequest

	if !httpapi.IsMultipartForm(c) {
		if err := c.ShouldBindJSON(&req); err != nil {
			apiresponse.WriteBindingError(c, err, req)
			return req, false
		}
		return req, true
	}

	payload, err := httpapi.MultipartPayload(c, "payload")
	if err != nil {
		apiresponse.WriteValidationError(c, err.Error())
		return req, false
	}

	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	// Process uploaded files and match them to media items by index
	for i := range req.Media {
		fileFieldName := "files"
		file, err := httpapi.ReadMultipartFile(c, fileFieldName)
		if err != nil || file == nil {
			continue
		}
		applyPressUploadedFile(&req.Media[i], file)
	}

	return req, true
}

func bindUpdatePressMediaRequest(c *gin.Context) (UpdatePressMediaRequest, bool) {
	var req UpdatePressMediaRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func bindDeletePressMediaRequest(c *gin.Context) (DeletePressMediaRequest, bool) {
	var req DeletePressMediaRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func bindReorderPressMediaRequest(c *gin.Context) (ReorderPressMediaRequest, bool) {
	var req ReorderPressMediaRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

// Helper functions

func applyPressUploadedFile(input *PressUploadInput, file *httpapi.UploadedFile) {
	if input == nil || file == nil {
		return
	}
	input.Content = file.Data
	input.FileName = file.Filename
	input.MimeType = file.ContentType
	input.FileSize = int64(len(file.Data))
}

func applyPressUploadedFilePtr(input **PressUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *input == nil {
		*input = &PressUploadInput{}
	}
	applyPressUploadedFile(*input, file)
}

func pressEntryUsesEmbeddedBase64(entry SavePressEntryRequest) bool {
	if entry.CoverImage != nil && strings.TrimSpace(entry.CoverImage.FileURL) != "" {
		return true
	}
	return false
}

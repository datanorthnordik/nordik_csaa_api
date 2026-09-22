package recordings

import (
	"encoding/json"
	"fmt"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveRecordingCollectionRequest(c *gin.Context) (SaveRecordingCollectionRequest, bool) {
	var req SaveRecordingCollectionRequest

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
		if saveRecordingCollectionRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		for idx := range req.Items {
			file, err := httpapi.ReadMultipartFile(c, recordingItemFileField(idx))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid multipart form data")
				return req, false
			}
			applyRecordingUploadedFile(&req.Items[idx], file)
		}
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if saveRecordingCollectionRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func bindAddRecordingItemsRequest(c *gin.Context) (AddRecordingItemsRequest, bool) {
	var req AddRecordingItemsRequest

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
		if addRecordingItemsRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		for idx := range req.Items {
			file, err := httpapi.ReadMultipartFile(c, recordingItemFileField(idx))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid multipart form data")
				return req, false
			}
			applyRecordingUploadedFile(&req.Items[idx], file)
		}
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if addRecordingItemsRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func bindUpdateRecordingItemRequest(c *gin.Context) (UpdateRecordingItemRequest, bool) {
	var req UpdateRecordingItemRequest

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
		if strings.TrimSpace(req.DataBase64) != "" {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		file, err := httpapi.ReadMultipartFile(c, "recording_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyRecordingUploadedFile(&req, file)
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if strings.TrimSpace(req.DataBase64) != "" {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func saveRecordingCollectionRequestUsesEmbeddedBase64(req SaveRecordingCollectionRequest) bool {
	for _, item := range req.Items {
		if strings.TrimSpace(item.DataBase64) != "" {
			return true
		}
	}
	return false
}

func addRecordingItemsRequestUsesEmbeddedBase64(req AddRecordingItemsRequest) bool {
	for _, item := range req.Items {
		if strings.TrimSpace(item.DataBase64) != "" {
			return true
		}
	}
	return false
}

func applyRecordingUploadedFile(dst *RecordingItemInput, file *httpapi.UploadedFile) {
	if dst == nil || file == nil {
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

func recordingItemFileField(idx int) string {
	return fmt.Sprintf("items[%d].recording_file", idx)
}

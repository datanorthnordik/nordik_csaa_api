package events

import (
	"encoding/json"
	"fmt"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveEventRequest(c *gin.Context) (SaveEventRequest, bool) {
	var req SaveEventRequest

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
		if saveEventRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		displayImageFile, err := httpapi.ReadMultipartFile(c, "display_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyEventUploadedFilePtr(&req.DisplayImage, displayImageFile)

		for idx := range req.Attachments {
			file, err := httpapi.ReadMultipartFile(c, eventAttachmentFileField(idx))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid multipart form data")
				return req, false
			}
			applyEventUploadedFile(&req.Attachments[idx], file)
		}

		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if saveEventRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func saveEventRequestUsesEmbeddedBase64(req SaveEventRequest) bool {
	if req.DisplayImage != nil && strings.TrimSpace(req.DisplayImage.DataBase64) != "" {
		return true
	}
	for _, attachment := range req.Attachments {
		if strings.TrimSpace(attachment.DataBase64) != "" {
			return true
		}
	}
	return false
}

func applyEventUploadedFile(dst *EventUploadInput, file *httpapi.UploadedFile) {
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

func applyEventUploadedFilePtr(dst **EventUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *dst == nil {
		*dst = &EventUploadInput{}
	}
	applyEventUploadedFile(*dst, file)
}

func eventAttachmentFileField(idx int) string {
	return fmt.Sprintf("attachments[%d].file", idx)
}

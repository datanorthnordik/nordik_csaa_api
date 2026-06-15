package video

import (
	"encoding/json"
	"fmt"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveVideoPackageRequest(c *gin.Context) (SaveVideoPackageRequest, bool) {
	var req SaveVideoPackageRequest

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
		if saveVideoPackageRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		if req.SingleVideo != nil {
			file, err := httpapi.ReadMultipartFile(c, "single_video.teaser_image_file")
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid multipart form data")
				return req, false
			}
			applyVideoUploadedFile(req.SingleVideo, file)
		}
		for idx := range req.Videos {
			file, err := httpapi.ReadMultipartFile(c, videoItemFileField(idx))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid multipart form data")
				return req, false
			}
			applyVideoUploadedFile(&req.Videos[idx], file)
		}
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if saveVideoPackageRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func bindAddVideoItemsRequest(c *gin.Context) (AddVideoItemsRequest, bool) {
	var req AddVideoItemsRequest

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
		if addVideoItemsRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		for idx := range req.Videos {
			file, err := httpapi.ReadMultipartFile(c, videoItemFileField(idx))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid multipart form data")
				return req, false
			}
			applyVideoUploadedFile(&req.Videos[idx], file)
		}
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if addVideoItemsRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func bindUpdateVideoItemRequest(c *gin.Context) (UpdateVideoItemRequest, bool) {
	var req UpdateVideoItemRequest

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
		if updateVideoItemRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		file, err := httpapi.ReadMultipartFile(c, "teaser_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyVideoUploadedFile(&req, file)
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if updateVideoItemRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func saveVideoPackageRequestUsesEmbeddedBase64(req SaveVideoPackageRequest) bool {
	if req.SingleVideo != nil && strings.TrimSpace(req.SingleVideo.DataBase64) != "" {
		return true
	}
	for _, item := range req.Videos {
		if strings.TrimSpace(item.DataBase64) != "" {
			return true
		}
	}
	return false
}

func addVideoItemsRequestUsesEmbeddedBase64(req AddVideoItemsRequest) bool {
	for _, item := range req.Videos {
		if strings.TrimSpace(item.DataBase64) != "" {
			return true
		}
	}
	return false
}

func updateVideoItemRequestUsesEmbeddedBase64(req UpdateVideoItemRequest) bool {
	return strings.TrimSpace(req.DataBase64) != ""
}

func applyVideoUploadedFile(dst *VideoInput, file *httpapi.UploadedFile) {
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

func videoItemFileField(idx int) string {
	return fmt.Sprintf("videos[%d].teaser_image_file", idx)
}

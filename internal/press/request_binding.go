package press

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		if pressEntryUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		file, err := readOptionalMultipartFile(c, "cover_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid cover image file")
			return req, false
		}
		applyPressUploadedFilePtr(&req.CoverImage, file)
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if pressEntryUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func bindAddPressMediaRequest(c *gin.Context) (AddPressMediaRequest, bool) {
	var req AddPressMediaRequest

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
		if addPressMediaRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		for i := range req.Media {
			file, err := readOptionalMultipartFile(c, pressMediaFileField(i))
			if err != nil {
				apiresponse.WriteValidationError(c, "invalid media file upload")
				return req, false
			}
			applyPressUploadedFile(&req.Media[i], file)
		}

		files, err := readMultipartFiles(c, "files", "files[]")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid media file upload")
			return req, false
		}

		nextTargetIndex := 0
		for _, file := range files {
			for nextTargetIndex < len(req.Media) && len(req.Media[nextTargetIndex].Content) > 0 {
				nextTargetIndex++
			}
			if nextTargetIndex >= len(req.Media) {
				req.Media = append(req.Media, PressUploadInput{})
			}
			applyPressUploadedFile(&req.Media[nextTargetIndex], file)
			nextTargetIndex++
		}

		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if addPressMediaRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
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

func readOptionalMultipartFile(c *gin.Context, field string) (*httpapi.UploadedFile, error) {
	return httpapi.ReadMultipartFile(c, field)
}

func readMultipartFiles(c *gin.Context, fields ...string) ([]*httpapi.UploadedFile, error) {
	form, err := c.MultipartForm()
	if err != nil {
		if err == http.ErrNotMultipart || err == http.ErrMissingFile {
			return nil, nil
		}
		return nil, err
	}

	files := make([]*httpapi.UploadedFile, 0)
	for _, field := range fields {
		for _, header := range form.File[field] {
			file, err := header.Open()
			if err != nil {
				return nil, err
			}

			data, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if readErr != nil {
				return nil, readErr
			}
			if closeErr != nil {
				return nil, closeErr
			}

			files = append(files, &httpapi.UploadedFile{
				Data:        data,
				Filename:    header.Filename,
				ContentType: detectUploadedContentType(header.Header.Get("Content-Type"), data),
			})
		}
	}

	return files, nil
}

func applyPressUploadedFile(input *PressUploadInput, file *httpapi.UploadedFile) {
	if input == nil || file == nil {
		return
	}

	input.Content = append([]byte(nil), file.Data...)
	if strings.TrimSpace(input.FileName) == "" {
		input.FileName = strings.TrimSpace(file.Filename)
	}
	if strings.TrimSpace(input.MimeType) == "" {
		input.MimeType = strings.TrimSpace(file.ContentType)
	}
	input.FileSize = int64(len(file.Data))
	input.FileURL = strings.TrimSpace(input.FileURL)

	if strings.TrimSpace(input.DisplayName) == "" {
		input.DisplayName = input.FileName
	}
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
	if entry.CoverImage == nil {
		return false
	}
	fileURL := strings.TrimSpace(entry.CoverImage.FileURL)
	return isEmbeddedDataURL(fileURL)
}

func addPressMediaRequestUsesEmbeddedBase64(req AddPressMediaRequest) bool {
	for _, media := range req.Media {
		if isEmbeddedDataURL(media.FileURL) {
			return true
		}
	}
	return false
}

func pressMediaFileField(idx int) string {
	return fmt.Sprintf("media[%d].file", idx)
}

func detectUploadedContentType(contentType string, data []byte) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" || strings.EqualFold(contentType, "application/octet-stream") {
		return http.DetectContentType(data)
	}
	return contentType
}

func isEmbeddedDataURL(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:")
}

package books

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const bookMultipartPayloadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveBookRequest(c *gin.Context) (SaveBookRequest, bool) {
	var req SaveBookRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func bindSaveBookVersionRequest(c *gin.Context) (SaveBookVersionRequest, bool) {
	var req SaveBookVersionRequest

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

		file, err := httpapi.ReadMultipartFile(c, "source_pdf_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBookUploadedFile(&req.SourcePDF, file)

		file, err = httpapi.ReadMultipartFile(c, "generated_pdf_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBookUploadedFile(&req.GeneratedPDF, file)

		file, err = httpapi.ReadMultipartFile(c, "content_template_pdf_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBookUploadedFile(&req.ContentTemplatePDF, file)

		file, err = httpapi.ReadMultipartFile(c, "content_image_template_pdf_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBookUploadedFile(&req.ContentImageTemplatePDF, file)

		file, err = httpapi.ReadMultipartFile(c, "section_template_pdf_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBookUploadedFile(&req.SectionTemplatePDF, file)

		return req, true
	}

	if err := bindJSONPayloadCompat(c, &req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func bindGeneratedPDFUploadRequest(c *gin.Context) (BookUploadInput, bool) {
	var req struct {
		GeneratedPDF *BookUploadInput `json:"generated_pdf"`
	}

	if httpapi.IsMultipartForm(c) {
		payload, err := httpapi.MultipartPayload(c, "payload")
		if err != nil {
			apiresponse.WriteValidationError(c, err.Error())
			return BookUploadInput{}, false
		}
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			apiresponse.WriteBindingError(c, err, req)
			return BookUploadInput{}, false
		}

		file, err := httpapi.ReadMultipartFile(c, "generated_pdf_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return BookUploadInput{}, false
		}
		applyBookUploadedFile(&req.GeneratedPDF, file)
	} else {
		if err := bindJSONPayloadCompat(c, &req); err != nil {
			apiresponse.WriteBindingError(c, err, req)
			return BookUploadInput{}, false
		}
	}

	if req.GeneratedPDF == nil {
		apiresponse.WriteValidationError(c, bookMultipartPayloadValidationMessage)
		return BookUploadInput{}, false
	}

	return *req.GeneratedPDF, true
}

func bindSaveBookSubmissionRequest(c *gin.Context) (SaveBookSubmissionRequest, bool) {
	var req SaveBookSubmissionRequest

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

		file, err := httpapi.ReadMultipartFile(c, "image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBookUploadedFile(&req.Image, file)

		return req, true
	}

	if err := bindJSONPayloadCompat(c, &req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func bindUpdateBookSubmissionRequest(c *gin.Context) (UpdateBookSubmissionRequest, bool) {
	var req UpdateBookSubmissionRequest

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

		file, err := httpapi.ReadMultipartFile(c, "image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBookUploadedFile(&req.Image, file)

		return req, true
	}

	if err := bindJSONPayloadCompat(c, &req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func bindReviewBookSubmissionRequest(c *gin.Context) (ReviewBookSubmissionRequest, bool) {
	var req ReviewBookSubmissionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}

	return req, true
}

func applyBookUploadedFile(dst **BookUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *dst == nil {
		*dst = &BookUploadInput{}
	}

	if strings.TrimSpace((*dst).FileName) == "" {
		(*dst).FileName = file.Filename
	}
	if strings.TrimSpace((*dst).MimeType) == "" {
		(*dst).MimeType = file.ContentType
	}

	(*dst).Content = append([]byte(nil), file.Data...)
}

// Accept both the canonical JSON body and the legacy UI shape that nests it under payload.
func bindJSONPayloadCompat(c *gin.Context, dst any) error {
	if c == nil || c.Request == nil {
		return io.EOF
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return io.EOF
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return err
	}

	payload := trimmed
	if rawPayload, ok := envelope["payload"]; ok && len(bytes.TrimSpace(rawPayload)) > 0 {
		var nested string
		if err := json.Unmarshal(rawPayload, &nested); err == nil {
			payload = []byte(nested)
		} else {
			payload = rawPayload
		}
	}

	return json.Unmarshal(payload, dst)
}

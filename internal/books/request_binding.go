package books

import (
	"encoding/json"
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

		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
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
		if err := c.ShouldBindJSON(&req); err != nil {
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

	if err := c.ShouldBindJSON(&req); err != nil {
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

	if err := c.ShouldBindJSON(&req); err != nil {
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

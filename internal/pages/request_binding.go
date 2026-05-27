package pages

import (
	"encoding/json"
	"fmt"
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

		if req.PageDetail != nil {
			for sectionIdx := range req.PageDetail.Sections {
				section := &req.PageDetail.Sections[sectionIdx]
				if section.CTABanner != nil {
					file, err := httpapi.ReadMultipartFile(c, pageCTABannerImageFileField(sectionIdx))
					if err != nil {
						apiresponse.WriteValidationError(c, "invalid multipart form data")
						return req, false
					}
					applyPageUploadedFile(&section.CTABanner.Image, file)
				}
				if section.Documents == nil {
					continue
				}
				for documentIdx := range section.Documents.Items {
					file, err := httpapi.ReadMultipartFile(c, pageDocumentFileField(sectionIdx, documentIdx))
					if err != nil {
						apiresponse.WriteValidationError(c, "invalid multipart form data")
						return req, false
					}
					applyPageDocumentUploadedFile(&section.Documents.Items[documentIdx], file)
				}
			}
		}
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
	if req.HeroImage != nil && strings.TrimSpace(req.HeroImage.DataBase64) != "" {
		return true
	}
	if req.PageDetail == nil {
		return false
	}
	for _, section := range req.PageDetail.Sections {
		if section.CTABanner != nil &&
			section.CTABanner.Image != nil &&
			strings.TrimSpace(section.CTABanner.Image.DataBase64) != "" {
			return true
		}
		if section.Documents == nil {
			continue
		}
		for _, item := range section.Documents.Items {
			if strings.TrimSpace(item.DataBase64) != "" {
				return true
			}
		}
	}
	return false
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

func applyPageDocumentUploadedFile(dst *PageDocumentInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}

	if strings.TrimSpace(dst.FileName) == "" {
		dst.FileName = file.Filename
	}
	if strings.TrimSpace(dst.OriginalFileName) == "" {
		dst.OriginalFileName = file.Filename
	}
	if strings.TrimSpace(dst.MimeType) == "" {
		dst.MimeType = file.ContentType
	}
	dst.DataBase64 = ""
	dst.Content = append([]byte(nil), file.Data...)
}

func pageDocumentFileField(sectionIdx int, documentIdx int) string {
	return fmt.Sprintf("page_detail.sections[%d].documents.items[%d].file", sectionIdx, documentIdx)
}

func pageCTABannerImageFileField(sectionIdx int) string {
	return fmt.Sprintf("page_detail.sections[%d].cta_banner.image.file", sectionIdx)
}

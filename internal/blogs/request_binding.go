package blogs

import (
	"encoding/json"
	"fmt"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

const multipartUploadValidationMessage = "use multipart/form-data with a payload field for file uploads"

func bindSaveBlogRequest(c *gin.Context) (SaveBlogRequest, bool) {
	var req SaveBlogRequest

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
		if blogRequestUsesEmbeddedBase64(req) {
			apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
			return req, false
		}

		file, err := httpapi.ReadMultipartFile(c, "cover_image_file")
		if err != nil {
			apiresponse.WriteValidationError(c, "invalid multipart form data")
			return req, false
		}
		applyBlogUploadedFile(&req.CoverImage, file)

		if req.BlogDetail != nil {
			for sectionIdx := range req.BlogDetail.Sections {
				section := &req.BlogDetail.Sections[sectionIdx]
				if section.Image != nil {
					file, err := httpapi.ReadMultipartFile(c, blogSectionImageFileField(sectionIdx))
					if err != nil {
						apiresponse.WriteValidationError(c, "invalid multipart form data")
						return req, false
					}
					applyBlogUploadedFile(&section.Image.Asset, file)
				}
				if section.Animation == nil {
					continue
				}
				for itemIdx := range section.Animation.Items {
					file, err := httpapi.ReadMultipartFile(c, blogAnimationItemImageFileField(sectionIdx, itemIdx))
					if err != nil {
						apiresponse.WriteValidationError(c, "invalid multipart form data")
						return req, false
					}
					applyBlogUploadedFile(&section.Animation.Items[itemIdx].Image, file)
				}
			}
		}

		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return req, false
	}
	if blogRequestUsesEmbeddedBase64(req) {
		apiresponse.WriteValidationError(c, multipartUploadValidationMessage)
		return req, false
	}

	return req, true
}

func blogRequestUsesEmbeddedBase64(req SaveBlogRequest) bool {
	if req.CoverImage != nil && strings.TrimSpace(req.CoverImage.DataBase64) != "" {
		return true
	}
	if req.BlogDetail == nil {
		return false
	}

	for _, section := range req.BlogDetail.Sections {
		if section.Image != nil &&
			section.Image.Asset != nil &&
			strings.TrimSpace(section.Image.Asset.DataBase64) != "" {
			return true
		}
		if section.Animation == nil {
			continue
		}
		for _, item := range section.Animation.Items {
			if item.Image != nil && strings.TrimSpace(item.Image.DataBase64) != "" {
				return true
			}
		}
	}

	return false
}

func applyBlogUploadedFile(dst **BlogUploadInput, file *httpapi.UploadedFile) {
	if file == nil {
		return
	}
	if *dst == nil {
		*dst = &BlogUploadInput{}
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

func blogSectionImageFileField(sectionIdx int) string {
	return fmt.Sprintf("blog_detail.sections[%d].image.asset.file", sectionIdx)
}

func blogAnimationItemImageFileField(sectionIdx int, itemIdx int) string {
	return fmt.Sprintf("blog_detail.sections[%d].animation.items[%d].image.file", sectionIdx, itemIdx)
}

package gallery

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type GalleryController struct {
	GalleryService GalleryServicePort
}

func (gc *GalleryController) CreateGallery(c *gin.Context) {
	var req SaveGalleryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	resp, err := gc.GalleryService.CreateGallery(req, authUserID(c))
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Gallery created successfully", "gallery": resp})
}

func (gc *GalleryController) UpdateGallery(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	var req SaveGalleryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	resp, err := gc.GalleryService.UpdateGallery(id, req, authUserID(c))
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gallery updated successfully", "gallery": resp})
}

func (gc *GalleryController) DeleteGallery(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := gc.GalleryService.DeleteGallery(id); err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gallery deleted successfully"})
}

func (gc *GalleryController) AddGalleryImages(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	var req AddGalleryImagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	resp, err := gc.GalleryService.AddGalleryImages(id, req, authUserID(c))
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Gallery images uploaded successfully", "uploadedCount": resp.DeletedCount})
}

func (gc *GalleryController) DeleteGalleryImages(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	storageURLs := splitQueryValues(c.QueryArray("storage_url")...)
	if len(storageURLs) == 0 && c.Request.ContentLength > 0 {
		var req DeleteGalleryImagesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			apiresponse.WriteBindingError(c, err, req)
			return
		}
		storageURLs = req.StorageURLs
	}
	storageURLs = splitQueryValues(storageURLs...)
	if len(storageURLs) == 0 {
		apiresponse.WriteValidationError(c, "at least one storage_url is required")
		return
	}

	resp, err := gc.GalleryService.DeleteGalleryImages(id, storageURLs)
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gallery images deleted successfully", "deletedCount": resp.DeletedCount})
}

func writeGalleryError(c *gin.Context, err error) {
	httpapi.HandleError(c, "gallery", err,
		httpapi.ServiceUnavailableRule("Gallery service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrGalleryNotFound, ErrGalleryImageNotFound),
		httpapi.ConflictRule("Unable to save gallery because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeGalleryError),
	)
}

func isClientSafeGalleryError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " are required"),
		strings.Contains(message, "missing both data_base64 and file_url"),
		strings.Contains(message, "only image uploads are supported"),
		strings.Contains(message, "at least one "):
		return true
	default:
		return false
	}
}

func pathInt(c *gin.Context, key string) (int, bool) {
	value, err := strconv.Atoi(c.Param(key))
	if err != nil {
		apiresponse.WritePathParamError(c, key)
		return 0, false
	}
	return value, true
}

func authUserID(c *gin.Context) *int {
	value, ok := c.Get("auth_user_id")
	if !ok {
		return nil
	}
	switch id := value.(type) {
	case int:
		return &id
	case int32:
		v := int(id)
		return &v
	case int64:
		v := int(id)
		return &v
	case float64:
		v := int(id)
		return &v
	default:
		return nil
	}
}

func splitQueryValues(values ...string) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
	}
	return parts
}

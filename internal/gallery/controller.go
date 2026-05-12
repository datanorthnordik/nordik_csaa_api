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

func (gc *GalleryController) ListGalleries(c *gin.Context) {
	resp, err := gc.GalleryService.ListGalleries()
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (gc *GalleryController) GetGallery(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := gc.GalleryService.GetGallery(id)
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (gc *GalleryController) GetGalleryCoverContent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := gc.GalleryService.GetGalleryCoverContent(id)
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName := sanitizeContentDispositionFilename(resp.FileName); fileName != "" {
		c.Header("Content-Disposition", "inline; filename="+strconv.Quote(fileName))
	}

	c.Data(http.StatusOK, contentType, resp.Content)
}

func (gc *GalleryController) GetGalleryImageContent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}
	imageID, ok := pathInt(c, "imageId")
	if !ok {
		return
	}

	resp, err := gc.GalleryService.GetGalleryImageContent(id, imageID)
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName := sanitizeContentDispositionFilename(resp.FileName); fileName != "" {
		c.Header("Content-Disposition", "inline; filename="+strconv.Quote(fileName))
	}

	c.Data(http.StatusOK, contentType, resp.Content)
}

func (gc *GalleryController) CreateGallery(c *gin.Context) {
	req, ok := bindSaveGalleryRequest(c)
	if !ok {
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

	req, ok := bindSaveGalleryRequest(c)
	if !ok {
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

	req, ok := bindAddGalleryImagesRequest(c)
	if !ok {
		return
	}

	resp, err := gc.GalleryService.AddGalleryImages(id, req, authUserID(c))
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Gallery images uploaded successfully", "uploadedCount": resp.UploadedCount})
}

func (gc *GalleryController) UpdateGalleryImage(c *gin.Context) {
	id, imageID, ok := pathGalleryAndImageIDs(c)
	if !ok {
		return
	}

	var req UpdateGalleryImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	resp, err := gc.GalleryService.UpdateGalleryImage(id, imageID, req)
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gallery image updated successfully", "image": resp})
}

func (gc *GalleryController) ReorderGalleryImages(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	var req ReorderGalleryImagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	resp, err := gc.GalleryService.ReorderGalleryImages(id, req.ImageIDs)
	if err != nil {
		writeGalleryError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gallery images reordered successfully", "updatedCount": resp.UpdatedCount})
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
		strings.Contains(message, "missing both uploaded file and file_url"),
		strings.Contains(message, "use multipart/form-data"),
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

func pathGalleryAndImageIDs(c *gin.Context) (int, int, bool) {
	id, ok := pathInt(c, "id")
	if !ok {
		return 0, 0, false
	}
	imageID, ok := pathInt(c, "imageId")
	if !ok {
		return 0, 0, false
	}
	return id, imageID, true
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

func sanitizeContentDispositionFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

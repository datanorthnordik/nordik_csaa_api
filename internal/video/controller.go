package video

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type VideoController struct {
	VideoService VideoServicePort
}

func (vc *VideoController) ListVideoPackages(c *gin.Context) {
	resp, err := vc.VideoService.ListVideoPackages()
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (vc *VideoController) GetVideoPackage(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := vc.VideoService.GetVideoPackage(id)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (vc *VideoController) GetVideoTeaserContent(c *gin.Context) {
	id, itemID, ok := pathVideoAndItemIDs(c)
	if !ok {
		return
	}

	resp, err := vc.VideoService.GetVideoTeaserContent(id, itemID)
	if err != nil {
		writeVideoError(c, err)
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

func (vc *VideoController) CreateVideoPackage(c *gin.Context) {
	req, ok := bindSaveVideoPackageRequest(c)
	if !ok {
		return
	}

	resp, err := vc.VideoService.CreateVideoPackage(req, authUserID(c))
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Video package created successfully", "video": resp})
}

func (vc *VideoController) UpdateVideoPackage(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	var req UpdateVideoPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	resp, err := vc.VideoService.UpdateVideoPackage(id, req, authUserID(c))
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video package updated successfully", "video": resp})
}

func (vc *VideoController) DeleteVideoPackage(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := vc.VideoService.DeleteVideoPackage(id); err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video package deleted successfully"})
}

func (vc *VideoController) AddVideoItems(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindAddVideoItemsRequest(c)
	if !ok {
		return
	}

	resp, err := vc.VideoService.AddVideoItems(id, req, authUserID(c))
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Video items uploaded successfully", "uploadedCount": resp.UploadedCount})
}

func (vc *VideoController) UpdateVideoItem(c *gin.Context) {
	id, itemID, ok := pathVideoAndItemIDs(c)
	if !ok {
		return
	}

	req, ok := bindUpdateVideoItemRequest(c)
	if !ok {
		return
	}

	resp, err := vc.VideoService.UpdateVideoItem(id, itemID, req, authUserID(c))
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video item updated successfully", "item": resp})
}

func (vc *VideoController) DeleteVideoItem(c *gin.Context) {
	id, itemID, ok := pathVideoAndItemIDs(c)
	if !ok {
		return
	}

	resp, err := vc.VideoService.DeleteVideoItem(id, itemID)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video item deleted successfully", "deletedCount": resp.DeletedCount})
}

func writeVideoError(c *gin.Context, err error) {
	httpapi.HandleError(c, "video", err,
		httpapi.ServiceUnavailableRule("Video service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrVideoPackageNotFound, ErrVideoItemNotFound),
		httpapi.ConflictRule("Unable to save video package because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeVideoError),
	)
}

func isClientSafeVideoError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " are required"),
		strings.Contains(message, "must be a valid youtube url"),
		strings.Contains(message, "use multipart/form-data"),
		strings.Contains(message, "only image uploads are supported"),
		strings.Contains(message, "collection packages only"),
		strings.Contains(message, "package_type"),
		strings.Contains(message, "teaser image"):
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

func pathVideoAndItemIDs(c *gin.Context) (int, int, bool) {
	id, ok := pathInt(c, "id")
	if !ok {
		return 0, 0, false
	}
	itemID, ok := pathInt(c, "itemId")
	if !ok {
		return 0, 0, false
	}
	return id, itemID, true
}

func sanitizeContentDispositionFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

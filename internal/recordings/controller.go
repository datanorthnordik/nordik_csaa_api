package recordings

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type RecordingController struct {
	RecordingService RecordingServicePort
}

func (rc *RecordingController) ListRecordingCollections(c *gin.Context) {
	resp, err := rc.RecordingService.ListRecordingCollections()
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (rc *RecordingController) GetRecordingCollection(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := rc.RecordingService.GetRecordingCollection(id)
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (rc *RecordingController) GetRecordingCollectionByTitle(c *gin.Context) {
	title := strings.TrimSpace(c.Query("title"))
	if title == "" {
		apiresponse.WriteValidationError(c, "title is required")
		return
	}

	resp, err := rc.RecordingService.GetRecordingCollectionByTitle(title)
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (rc *RecordingController) GetRecordingCollectionByPlacementKey(c *gin.Context) {
	placementKey := strings.TrimSpace(c.Param("placementKey"))
	if placementKey == "" {
		apiresponse.WritePathParamError(c, "placementKey")
		return
	}

	resp, err := rc.RecordingService.GetRecordingCollectionByPlacementKey(placementKey)
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (rc *RecordingController) GetRecordingItemContent(c *gin.Context) {
	id, itemID, ok := pathRecordingCollectionAndItemIDs(c)
	if !ok {
		return
	}

	resp, err := rc.RecordingService.GetRecordingItemContent(id, itemID)
	if err != nil {
		writeRecordingError(c, err)
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

func (rc *RecordingController) CreateRecordingCollection(c *gin.Context) {
	req, ok := bindSaveRecordingCollectionRequest(c)
	if !ok {
		return
	}

	resp, err := rc.RecordingService.CreateRecordingCollection(req, authUserID(c))
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Recording collection created successfully", "recording": resp})
}

func (rc *RecordingController) UpdateRecordingCollection(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	var req UpdateRecordingCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	resp, err := rc.RecordingService.UpdateRecordingCollection(id, req, authUserID(c))
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Recording collection updated successfully", "recording": resp})
}

func (rc *RecordingController) DeleteRecordingCollection(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := rc.RecordingService.DeleteRecordingCollection(id); err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Recording collection deleted successfully"})
}

func (rc *RecordingController) AddRecordingItems(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindAddRecordingItemsRequest(c)
	if !ok {
		return
	}

	resp, err := rc.RecordingService.AddRecordingItems(id, req, authUserID(c))
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Recording items added successfully", "uploadedCount": resp.UploadedCount})
}

func (rc *RecordingController) UpdateRecordingItem(c *gin.Context) {
	id, itemID, ok := pathRecordingCollectionAndItemIDs(c)
	if !ok {
		return
	}

	req, ok := bindUpdateRecordingItemRequest(c)
	if !ok {
		return
	}

	resp, err := rc.RecordingService.UpdateRecordingItem(id, itemID, req, authUserID(c))
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Recording item updated successfully", "item": resp})
}

func (rc *RecordingController) DeleteRecordingItem(c *gin.Context) {
	id, itemID, ok := pathRecordingCollectionAndItemIDs(c)
	if !ok {
		return
	}

	resp, err := rc.RecordingService.DeleteRecordingItem(id, itemID)
	if err != nil {
		writeRecordingError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Recording item deleted successfully", "deletedCount": resp.DeletedCount})
}

func writeRecordingError(c *gin.Context, err error) {
	httpapi.HandleError(c, "recording", err,
		httpapi.ServiceUnavailableRule("Recording service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrRecordingCollectionNotFound, ErrRecordingItemNotFound),
		httpapi.ConflictRule("Unable to save recording collection because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeRecordingError),
	)
}

func isClientSafeRecordingError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " are required"),
		strings.Contains(message, "characters or fewer"),
		strings.Contains(message, "use multipart/form-data"),
		strings.Contains(message, "audio recording uploads"):
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

func pathRecordingCollectionAndItemIDs(c *gin.Context) (int, int, bool) {
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

package newsletters

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type NewsletterController struct {
	NewsletterService NewsletterServicePort
}

func (nc *NewsletterController) ListNewsletterEntries(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	filter := ListNewsletterFilter{
		Status:     strings.TrimSpace(c.DefaultQuery("status", "")),
		Visibility: strings.TrimSpace(c.DefaultQuery("visibility", "")),
		SearchTerm: strings.TrimSpace(c.DefaultQuery("search", "")),
		SortBy:     strings.TrimSpace(c.DefaultQuery("sort_by", "send_date")),
		SortOrder:  strings.TrimSpace(c.DefaultQuery("sort_order", "desc")),
		Page:       queryInt(c, "page", 1, 1, 0),
		PageSize:   queryInt(c, "page_size", 20, 1, 100),
	}

	resp, err := nc.NewsletterService.ListNewsletterEntries(filter)
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (nc *NewsletterController) GetNewsletterEntry(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.GetNewsletterEntry(id)
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (nc *NewsletterController) GetNewsletterMediaContent(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}
	mediaID, ok := pathInt(c, "mediaId")
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.GetNewsletterMediaContent(id, mediaID)
	if err != nil {
		writeNewsletterError(c, err)
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

func (nc *NewsletterController) CreateNewsletterEntry(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	req, ok := bindSaveNewsletterEntryRequest(c)
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.CreateNewsletterEntry(req, authUserID(c))
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Newsletter entry created successfully",
		"entry":   resp,
	})
}

func (nc *NewsletterController) UpdateNewsletterEntry(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindSaveNewsletterEntryRequest(c)
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.UpdateNewsletterEntry(id, req, authUserID(c))
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Newsletter entry updated successfully",
		"entry":   resp,
	})
}

func (nc *NewsletterController) DeleteNewsletterEntry(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := nc.NewsletterService.DeleteNewsletterEntry(id); err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Newsletter entry deleted successfully"})
}

func (nc *NewsletterController) AddNewsletterMedia(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindAddNewsletterMediaRequest(c)
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.AddNewsletterMedia(id, req, authUserID(c))
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "Newsletter media added successfully",
		"uploadedCount": resp.UploadedCount,
	})
}

func (nc *NewsletterController) UpdateNewsletterMedia(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}
	mediaID, ok := pathInt(c, "mediaId")
	if !ok {
		return
	}

	req, ok := bindUpdateNewsletterMediaRequest(c)
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.UpdateNewsletterMedia(id, mediaID, req)
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Newsletter media updated successfully",
		"media":   resp,
	})
}

func (nc *NewsletterController) ReorderNewsletterMedia(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindReorderNewsletterMediaRequest(c)
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.ReorderNewsletterMedia(id, req.MediaIDs)
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Newsletter media reordered successfully",
		"updatedCount": resp.UpdatedCount,
	})
}

func (nc *NewsletterController) DeleteNewsletterMedia(c *gin.Context) {
	if nc.NewsletterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindDeleteNewsletterMediaRequest(c)
	if !ok {
		return
	}

	resp, err := nc.NewsletterService.DeleteNewsletterMedia(id, req.MediaIDs)
	if err != nil {
		writeNewsletterError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Newsletter media deleted successfully",
		"deletedCount": resp.DeletedCount,
	})
}

func queryInt(c *gin.Context, key string, fallback int, min int, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery(key, strconv.Itoa(fallback))))
	if err != nil || value < min || (max > 0 && value > max) {
		return fallback
	}
	return value
}

func pathInt(c *gin.Context, param string) (int, bool) {
	value := strings.TrimSpace(c.Param(param))
	if value == "" {
		apiresponse.WritePathParamError(c, param)
		return 0, false
	}

	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		apiresponse.WritePathParamError(c, param)
		return 0, false
	}

	return id, true
}

func authUserID(c *gin.Context) *int {
	if c == nil {
		return nil
	}
	for _, key := range []string{"auth_user_id", "userID", "user_id", "userId"} {
		val, exists := c.Get(key)
		if !exists {
			continue
		}
		switch v := val.(type) {
		case int:
			return &v
		case int32:
			userID := int(v)
			return &userID
		case int64:
			userID := int(v)
			return &userID
		case uint:
			userID := int(v)
			return &userID
		case float64:
			userID := int(v)
			return &userID
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func sanitizeContentDispositionFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")
	filename = strings.ReplaceAll(filename, "\r", "")
	filename = strings.ReplaceAll(filename, "\n", "")
	filename = strings.ReplaceAll(filename, ";", "")
	filename = strings.ReplaceAll(filename, "\"", "")
	return filename
}

func writeNewsletterError(c *gin.Context, err error) {
	httpapi.HandleError(c, "newsletters", err,
		httpapi.ServiceUnavailableRule("Newsletter service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrNewsletterEntryNotFound, ErrNewsletterMediaNotFound),
		httpapi.ConflictRule("Unable to save newsletter because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeNewsletterError),
	)
}

func isClientSafeNewsletterError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " must be "),
		strings.Contains(message, " must contain "),
		strings.Contains(message, " must not contain "),
		strings.Contains(message, "invalid "),
		strings.Contains(message, "file upload or file_url is required"),
		strings.Contains(message, "media content is not available from storage"),
		strings.Contains(message, "use multipart/form-data"),
		strings.Contains(message, "must include every newsletter media item exactly once"):
		return true
	default:
		return false
	}
}

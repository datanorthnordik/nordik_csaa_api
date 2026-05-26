package memorial

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type MemorialController struct {
	MemorialService MemorialServicePort
}

func (mc *MemorialController) ListMemorials(c *gin.Context) {
	if mc.MemorialService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	filter := ListMemorialsFilter{
		Page:       queryInt(c, "page", 1, 1, 0),
		PageSize:   queryInt(c, "page_size", 10, 1, 100),
		SearchTerm: strings.TrimSpace(c.Query("search")),
		Status:     strings.TrimSpace(c.DefaultQuery("status", "all")),
		Category:   strings.TrimSpace(c.Query("category")),
		SortBy:     strings.TrimSpace(c.DefaultQuery("sort_by", "date_of_passing")),
		SortOrder:  strings.TrimSpace(c.DefaultQuery("sort_order", "desc")),
		PublicOnly: true,
	}

	resp, err := mc.MemorialService.ListMemorials(filter)
	if err != nil {
		writeMemorialError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (mc *MemorialController) GetMemorial(c *gin.Context) {
	if mc.MemorialService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := mc.MemorialService.GetMemorial(id)
	if err != nil {
		writeMemorialError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (mc *MemorialController) GetMemorialPortraitContent(c *gin.Context) {
	if mc.MemorialService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := mc.MemorialService.GetMemorialPortraitContent(id)
	if err != nil {
		writeMemorialError(c, err)
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

func (mc *MemorialController) GetMemorialGalleryImageContent(c *gin.Context) {
	if mc.MemorialService == nil {
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

	resp, err := mc.MemorialService.GetMemorialGalleryImageContent(id, mediaID)
	if err != nil {
		writeMemorialError(c, err)
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

func (mc *MemorialController) CreateMemorial(c *gin.Context) {
	if mc.MemorialService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	req, ok := bindSaveMemorialRequest(c)
	if !ok {
		return
	}

	resp, err := mc.MemorialService.CreateMemorial(req, authUserID(c))
	if err != nil {
		writeMemorialError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Memorial entry created successfully",
		"memorial": resp,
	})
}

func (mc *MemorialController) UpdateMemorial(c *gin.Context) {
	if mc.MemorialService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindSaveMemorialRequest(c)
	if !ok {
		return
	}

	resp, err := mc.MemorialService.UpdateMemorial(id, req, authUserID(c))
	if err != nil {
		writeMemorialError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Memorial entry updated successfully",
		"memorial": resp,
	})
}

func (mc *MemorialController) DeleteMemorial(c *gin.Context) {
	if mc.MemorialService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := mc.MemorialService.DeleteMemorial(id); err != nil {
		writeMemorialError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Memorial entry deleted successfully"})
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

func writeMemorialError(c *gin.Context, err error) {
	httpapi.HandleError(c, "memorial", err,
		httpapi.ServiceUnavailableRule("Memorial service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrMemorialNotFound, ErrMemorialMediaNotFound),
		httpapi.ConflictRule("Unable to save memorial entry because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeMemorialError),
	)
}

func isClientSafeMemorialError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " must be "),
		strings.Contains(message, " must not "),
		strings.Contains(message, "only image uploads are supported"),
		strings.Contains(message, "memorial content is not available from storage"),
		strings.Contains(message, "use multipart/form-data"):
		return true
	default:
		return false
	}
}

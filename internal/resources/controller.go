package resources

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type ResourceController struct {
	ResourceService ResourceServicePort
}

func (rc *ResourceController) ListResources(c *gin.Context) {
	if rc.ResourceService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	filter := ListResourcesFilter{
		Page:       queryInt(c, "page", 1, 1, 0),
		PageSize:   queryInt(c, "page_size", 10, 1, 100),
		SearchTerm: strings.TrimSpace(c.Query("search")),
		Category:   strings.TrimSpace(c.Query("category")),
		FileType:   strings.TrimSpace(c.DefaultQuery("file_type", "all")),
	}

	resp, err := rc.ResourceService.ListResources(filter)
	if err != nil {
		writeResourceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (rc *ResourceController) GetResource(c *gin.Context) {
	if rc.ResourceService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := rc.ResourceService.GetResource(id)
	if err != nil {
		writeResourceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (rc *ResourceController) GetResourceContent(c *gin.Context) {
	if rc.ResourceService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := rc.ResourceService.GetResourceContent(id)
	if err != nil {
		writeResourceError(c, err)
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

func (rc *ResourceController) CreateResource(c *gin.Context) {
	if rc.ResourceService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	req, ok := bindSaveResourceRequest(c)
	if !ok {
		return
	}

	resp, err := rc.ResourceService.CreateResource(req, authUserID(c))
	if err != nil {
		writeResourceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Resource created successfully",
		"resource": resp,
	})
}

func (rc *ResourceController) UpdateResource(c *gin.Context) {
	if rc.ResourceService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindSaveResourceRequest(c)
	if !ok {
		return
	}

	resp, err := rc.ResourceService.UpdateResource(id, req, authUserID(c))
	if err != nil {
		writeResourceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Resource updated successfully",
		"resource": resp,
	})
}

func (rc *ResourceController) DeleteResource(c *gin.Context) {
	if rc.ResourceService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := rc.ResourceService.DeleteResource(id); err != nil {
		writeResourceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Resource deleted successfully"})
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

func writeResourceError(c *gin.Context, err error) {
	httpapi.HandleError(c, "resources", err,
		httpapi.ServiceUnavailableRule("Resource service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrResourceNotFound),
		httpapi.ConflictRule("Unable to save resource because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeResourceError),
	)
}

func isClientSafeResourceError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " must be "),
		strings.Contains(message, " must be one of "),
		strings.Contains(message, "file_type is invalid"),
		strings.Contains(message, "resource content is not available from storage"),
		strings.Contains(message, "use multipart/form-data"):
		return true
	default:
		return false
	}
}

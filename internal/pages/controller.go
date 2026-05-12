package pages

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type PageController struct {
	PageService PageServicePort
}

func (pc *PageController) ListPages(c *gin.Context) {
	filter := PageListFilters{
		Page:       parseQueryInt(c.Query("page")),
		PageSize:   parseQueryInt(c.Query("page_size")),
		SearchTerm: c.Query("search"),
		Status:     c.Query("status"),
		SortBy:     c.DefaultQuery("sort_by", "last_modified"),
		SortOrder:  c.DefaultQuery("sort_order", "desc"),
	}

	resp, err := pc.PageService.ListPages(filter)
	if err != nil {
		writePageError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (pc *PageController) GetPage(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := pc.PageService.GetPage(id)
	if err != nil {
		writePageError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (pc *PageController) GetPageHeroImageContent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := pc.PageService.GetPageHeroImageContent(id)
	if err != nil {
		writePageError(c, err)
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

func (pc *PageController) CreatePage(c *gin.Context) {
	req, ok := bindSavePageRequest(c)
	if !ok {
		return
	}

	if authUserID := authUserIDFromContext(c); authUserID != nil {
		req.CreatedBy = authUserID
		req.ModifiedBy = authUserID
	}

	resp, err := pc.PageService.CreatePage(req)
	if err != nil {
		writePageError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Page created successfully",
		"page":    resp,
	})
}

func (pc *PageController) UpdatePage(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindSavePageRequest(c)
	if !ok {
		return
	}

	if authUserID := authUserIDFromContext(c); authUserID != nil {
		req.ModifiedBy = authUserID
	}

	resp, err := pc.PageService.UpdatePage(id, req)
	if err != nil {
		writePageError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Page updated successfully",
		"page":    resp,
	})
}

func (pc *PageController) DeletePage(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := pc.PageService.DeletePage(id); err != nil {
		writePageError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Page deleted successfully"})
}

func writePageError(c *gin.Context, err error) {
	httpapi.HandleError(c, "pages", err,
		httpapi.ServiceUnavailableRule("Page service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrPageNotFound, ErrPageHeroImageNotFound),
		httpapi.ConflictRule("Unable to save page because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafePageError),
	)
}

func isClientSafePageError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, "invalid "),
		strings.Contains(message, "must be a valid"),
		strings.Contains(message, "missing both uploaded file and file_url"):
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

func parseQueryInt(value string) int {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return parsed
}

func sanitizeContentDispositionFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

func authUserIDFromContext(c *gin.Context) *int {
	value, exists := c.Get("auth_user_id")
	if !exists {
		return nil
	}

	switch typed := value.(type) {
	case int:
		return &typed
	case int32:
		next := int(typed)
		return &next
	case int64:
		next := int(typed)
		return &next
	default:
		return nil
	}
}

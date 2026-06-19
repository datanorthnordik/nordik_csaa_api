package blogs

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type BlogController struct {
	BlogService BlogServicePort
}

func (bc *BlogController) ListBlogs(c *gin.Context) {
	pageValue, hasPage := c.GetQuery("page")
	pageSizeValue, hasPageSize := c.GetQuery("page_size")

	filter := BlogListFilters{
		Page:          parseQueryInt(pageValue),
		PageSize:      parseQueryInt(pageSizeValue),
		SearchTerm:    c.Query("search"),
		SortBy:        c.DefaultQuery("sort_by", "publish_date"),
		SortOrder:     c.DefaultQuery("sort_order", "desc"),
		UsePagination: hasPage || hasPageSize,
	}

	resp, err := bc.BlogService.ListBlogs(filter)
	if err != nil {
		writeBlogError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (bc *BlogController) GetBlog(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := bc.BlogService.GetBlog(id)
	if err != nil {
		writeBlogError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (bc *BlogController) GetBlogCoverImageContent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := bc.BlogService.GetBlogCoverImageContent(id)
	if err != nil {
		writeBlogError(c, err)
		return
	}

	writeBlogMediaContent(c, resp)
}

func (bc *BlogController) GetBlogSectionImageContent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	sectionID, ok := pathInt(c, "sectionId")
	if !ok {
		return
	}

	resp, err := bc.BlogService.GetBlogSectionImageContent(id, sectionID)
	if err != nil {
		writeBlogError(c, err)
		return
	}

	writeBlogMediaContent(c, resp)
}

func (bc *BlogController) GetBlogAnimationItemImageContent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	sectionID, ok := pathInt(c, "sectionId")
	if !ok {
		return
	}
	itemID, ok := pathInt(c, "itemId")
	if !ok {
		return
	}

	resp, err := bc.BlogService.GetBlogAnimationItemImageContent(id, sectionID, itemID)
	if err != nil {
		writeBlogError(c, err)
		return
	}

	writeBlogMediaContent(c, resp)
}

func (bc *BlogController) CreateBlog(c *gin.Context) {
	req, ok := bindSaveBlogRequest(c)
	if !ok {
		return
	}

	if authUserID := authUserIDFromContext(c); authUserID != nil {
		req.CreatedBy = authUserID
		req.UpdatedBy = authUserID
	}

	resp, err := bc.BlogService.CreateBlog(req)
	if err != nil {
		writeBlogError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Blog created successfully",
		"blog":    resp,
	})
}

func (bc *BlogController) UpdateBlog(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindSaveBlogRequest(c)
	if !ok {
		return
	}

	if authUserID := authUserIDFromContext(c); authUserID != nil {
		req.UpdatedBy = authUserID
	}

	resp, err := bc.BlogService.UpdateBlog(id, req)
	if err != nil {
		writeBlogError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Blog updated successfully",
		"blog":    resp,
	})
}

func (bc *BlogController) DeleteBlog(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := bc.BlogService.DeleteBlog(id); err != nil {
		writeBlogError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Blog deleted successfully"})
}

func writeBlogMediaContent(c *gin.Context, resp *BlogMediaContent) {
	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName := sanitizeContentDispositionFilename(resp.FileName); fileName != "" {
		c.Header("Content-Disposition", "inline; filename="+strconv.Quote(fileName))
	}
	c.Data(http.StatusOK, contentType, resp.Content)
}

func writeBlogError(c *gin.Context, err error) {
	httpapi.HandleError(c, "blogs", err,
		httpapi.ServiceUnavailableRule("Blog service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrBlogNotFound, ErrBlogCoverImageNotFound, ErrBlogSectionImageNotFound, ErrBlogAnimationItemImageNotFound),
		httpapi.ConflictRule("Unable to save blog because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeBlogError),
	)
}

func isClientSafeBlogError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, "invalid "),
		strings.Contains(message, "must be "),
		strings.Contains(message, "missing both uploaded file and file_url"),
		strings.Contains(message, "blog_detail.sections"):
		return true
	default:
		return false
	}
}

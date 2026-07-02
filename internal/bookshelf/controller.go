package bookshelf

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type BookshelfController struct {
	BookshelfService BookshelfServicePort
}

func (bc *BookshelfController) ListBooks(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	filter := ListBookshelfFilter{
		Page:       bookshelfQueryInt(c, "page", 1, 1, 0),
		PageSize:   bookshelfQueryInt(c, "page_size", 10, 1, 100),
		SearchTerm: strings.TrimSpace(c.Query("search")),
	}

	resp, err := bc.BookshelfService.ListBooks(filter)
	if err != nil {
		writeBookshelfError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (bc *BookshelfController) GetBook(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := bookshelfPathInt(c, "id")
	if !ok {
		return
	}

	resp, err := bc.BookshelfService.GetBook(id)
	if err != nil {
		writeBookshelfError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (bc *BookshelfController) GetBookContent(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := bookshelfPathInt(c, "id")
	if !ok {
		return
	}

	resp, err := bc.BookshelfService.GetBookContent(id)
	if err != nil {
		writeBookshelfError(c, err)
		return
	}

	bookshelfWriteContent(c, resp)
}

func (bc *BookshelfController) GetCoverImageContent(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := bookshelfPathInt(c, "id")
	if !ok {
		return
	}

	resp, err := bc.BookshelfService.GetCoverImageContent(id)
	if err != nil {
		writeBookshelfError(c, err)
		return
	}

	bookshelfWriteContent(c, resp)
}

func (bc *BookshelfController) GetAuthorImageContent(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := bookshelfPathInt(c, "id")
	if !ok {
		return
	}

	resp, err := bc.BookshelfService.GetAuthorImageContent(id)
	if err != nil {
		writeBookshelfError(c, err)
		return
	}

	bookshelfWriteContent(c, resp)
}

func (bc *BookshelfController) CreateBook(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	req, ok := bindSaveBookshelfEntryRequest(c)
	if !ok {
		return
	}

	resp, err := bc.BookshelfService.CreateBook(req, bookshelfAuthUserID(c))
	if err != nil {
		writeBookshelfError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Book created successfully",
		"book":    resp,
	})
}

func (bc *BookshelfController) UpdateBook(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := bookshelfPathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindSaveBookshelfEntryRequest(c)
	if !ok {
		return
	}

	resp, err := bc.BookshelfService.UpdateBook(id, req, bookshelfAuthUserID(c))
	if err != nil {
		writeBookshelfError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Book updated successfully",
		"book":    resp,
	})
}

func (bc *BookshelfController) DeleteBook(c *gin.Context) {
	if bc.BookshelfService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	id, ok := bookshelfPathInt(c, "id")
	if !ok {
		return
	}

	if err := bc.BookshelfService.DeleteBook(id); err != nil {
		writeBookshelfError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Book deleted successfully"})
}

func writeBookshelfError(c *gin.Context, err error) {
	httpapi.HandleError(c, "bookshelf", err,
		httpapi.ServiceUnavailableRule("Bookshelf service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrBookNotFound, ErrBookContentNotFound, ErrAuthorImageNotFound, ErrCoverImageNotFound),
		httpapi.ConflictRule("Unable to save book because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeBookshelfError),
	)
}

func isClientSafeBookshelfError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " must be "),
		strings.Contains(message, " must not "),
		strings.Contains(message, "use multipart/form-data"),
		strings.Contains(message, "book upload returned an empty file url"),
		strings.Contains(message, "author image upload returned an empty file url"),
		strings.Contains(message, "cover image upload returned an empty file url"),
		strings.Contains(message, "book content is not available from storage"),
		strings.Contains(message, "author image is not available from storage"),
		strings.Contains(message, "cover image is not available from storage"),
		strings.Contains(message, "book does not have downloadable content"),
		strings.Contains(message, "book does not have an author image"),
		strings.Contains(message, "book does not have a cover image"):
		return true
	default:
		return false
	}
}

func bookshelfWriteContent(c *gin.Context, resp *BookshelfContent) {
	if resp == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName := sanitizeBookshelfContentDispositionFilename(resp.FileName); fileName != "" {
		c.Header("Content-Disposition", "inline; filename="+strconv.Quote(fileName))
	}

	c.Data(http.StatusOK, contentType, resp.Content)
}

func bookshelfQueryInt(c *gin.Context, key string, fallback int, min int, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery(key, strconv.Itoa(fallback))))
	if err != nil || value < min || (max > 0 && value > max) {
		return fallback
	}
	return value
}

func bookshelfPathInt(c *gin.Context, param string) (int, bool) {
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

func bookshelfAuthUserID(c *gin.Context) *int {
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

func sanitizeBookshelfContentDispositionFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")
	filename = strings.ReplaceAll(filename, "\r", "")
	filename = strings.ReplaceAll(filename, "\n", "")
	filename = strings.ReplaceAll(filename, ";", "")
	filename = strings.ReplaceAll(filename, "\"", "")
	return filename
}

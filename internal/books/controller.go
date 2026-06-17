package books

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type BookController struct {
	BookService BookServicePort
}

func (bc *BookController) ListBooks(c *gin.Context) {
	resp, err := bc.BookService.ListBooks()
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"books": resp})
}

func (bc *BookController) GetBook(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetBook(bookID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"book": resp})
}

func (bc *BookController) CreateBook(c *gin.Context) {
	req, ok := bindSaveBookRequest(c)
	if !ok {
		return
	}

	if userID := bookAuthUserIDFromContext(c); userID != nil {
		req.CreatedBy = userID
		req.UpdatedBy = userID
	}

	resp, err := bc.BookService.CreateBook(req)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Book created successfully",
		"book":    resp,
	})
}

func (bc *BookController) UpdateBook(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}

	req, ok := bindSaveBookRequest(c)
	if !ok {
		return
	}

	if userID := bookAuthUserIDFromContext(c); userID != nil {
		req.UpdatedBy = userID
	}

	resp, err := bc.BookService.UpdateBook(bookID, req)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Book updated successfully",
		"book":    resp,
	})
}

func (bc *BookController) CreateBookVersion(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}

	req, ok := bindSaveBookVersionRequest(c)
	if !ok {
		return
	}

	if userID := bookAuthUserIDFromContext(c); userID != nil {
		req.CreatedBy = userID
		req.UpdatedBy = userID
	}

	resp, err := bc.BookService.CreateBookVersion(bookID, req)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Book version created successfully",
		"version": resp,
	})
}

func (bc *BookController) UpdateBookVersion(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	versionID, ok := bookPathInt(c, "versionId")
	if !ok {
		return
	}

	req, ok := bindSaveBookVersionRequest(c)
	if !ok {
		return
	}

	if userID := bookAuthUserIDFromContext(c); userID != nil {
		req.UpdatedBy = userID
	}

	resp, err := bc.BookService.UpdateBookVersion(bookID, versionID, req)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Book version updated successfully",
		"version": resp,
	})
}

func (bc *BookController) SetActiveVersion(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	versionID, ok := bookPathInt(c, "versionId")
	if !ok {
		return
	}

	resp, err := bc.BookService.SetActiveVersion(bookID, versionID, bookAuthUserIDFromContext(c))
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Book version activated successfully",
		"version": resp,
	})
}

func (bc *BookController) GetBookVersionDetail(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	versionID, ok := bookPathInt(c, "versionId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetBookVersionDetail(bookID, versionID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"version": resp})
}

func (bc *BookController) UploadGeneratedPDF(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	versionID, ok := bookPathInt(c, "versionId")
	if !ok {
		return
	}

	input, ok := bindGeneratedPDFUploadRequest(c)
	if !ok {
		return
	}

	resp, err := bc.BookService.UploadGeneratedPDF(bookID, versionID, input, bookAuthUserIDFromContext(c))
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Generated PDF uploaded successfully",
		"version": resp,
	})
}

func (bc *BookController) GetSourcePDFContent(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	versionID, ok := bookPathInt(c, "versionId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetSourcePDFContent(bookID, versionID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	bookWriteBinary(c, resp)
}

func (bc *BookController) GetGeneratedPDFContent(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	versionID, ok := bookPathInt(c, "versionId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetGeneratedPDFContent(bookID, versionID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	bookWriteBinary(c, resp)
}

func (bc *BookController) ListBookSubmissions(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}

	filter := ListBookSubmissionsFilter{
		VersionID: bookParseQueryInt(c.Query("version_id")),
		Status:    strings.TrimSpace(strings.ToLower(c.Query("status"))),
	}

	resp, err := bc.BookService.ListBookSubmissions(bookID, filter)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"submissions": resp})
}

func (bc *BookController) GetBookSubmission(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	submissionID, ok := bookPathInt(c, "submissionId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetBookSubmission(bookID, submissionID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"submission": resp})
}

func (bc *BookController) CreatePublicSubmission(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}

	req, ok := bindSaveBookSubmissionRequest(c)
	if !ok {
		return
	}

	resp, err := bc.BookService.CreatePublicSubmission(bookID, req)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Book submission created successfully",
		"submission": resp,
	})
}

func (bc *BookController) UpdateBookSubmission(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	submissionID, ok := bookPathInt(c, "submissionId")
	if !ok {
		return
	}

	req, ok := bindUpdateBookSubmissionRequest(c)
	if !ok {
		return
	}

	resp, err := bc.BookService.UpdateBookSubmission(bookID, submissionID, req)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Book submission updated successfully",
		"submission": resp,
	})
}

func (bc *BookController) ApproveBookSubmission(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	submissionID, ok := bookPathInt(c, "submissionId")
	if !ok {
		return
	}

	resp, err := bc.BookService.ApproveBookSubmission(bookID, submissionID, bookAuthUserIDFromContext(c))
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Book submission approved successfully",
		"submission": resp,
	})
}

func (bc *BookController) RejectBookSubmission(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	submissionID, ok := bookPathInt(c, "submissionId")
	if !ok {
		return
	}

	req, ok := bindReviewBookSubmissionRequest(c)
	if !ok {
		return
	}
	if userID := bookAuthUserIDFromContext(c); userID != nil {
		req.ReviewedBy = userID
	}

	resp, err := bc.BookService.RejectBookSubmission(bookID, submissionID, req)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Book submission rejected successfully",
		"submission": resp,
	})
}

func (bc *BookController) GetSubmissionImageContent(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}
	submissionID, ok := bookPathInt(c, "submissionId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetSubmissionImageContent(bookID, submissionID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName := sanitizeBookContentDispositionFilename(resp.FileName); fileName != "" {
		c.Header("Content-Disposition", "inline; filename="+strconv.Quote(fileName))
	}

	c.Data(http.StatusOK, contentType, resp.Content)
}

func (bc *BookController) ListPublicBooks(c *gin.Context) {
	resp, err := bc.BookService.ListPublicBooks()
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"books": resp})
}

func (bc *BookController) GetPublicBook(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetPublicBook(bookID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"book": resp})
}

func (bc *BookController) GetPublicActivePDFContent(c *gin.Context) {
	bookID, ok := bookPathInt(c, "bookId")
	if !ok {
		return
	}

	resp, err := bc.BookService.GetPublicActivePDFContent(bookID)
	if err != nil {
		writeBookError(c, err)
		return
	}

	bookWriteBinary(c, resp)
}

func writeBookError(c *gin.Context, err error) {
	httpapi.HandleError(c, "books", err,
		httpapi.ServiceUnavailableRule("Book service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(
			ErrBookNotFound,
			ErrBookVersionNotFound,
			ErrBookSubmissionNotFound,
			ErrBookActiveVersionNotFound,
			ErrBookPDFNotFound,
			ErrBookSubmissionImageNotFound,
		),
		httpapi.ConflictRule("Unable to save book because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeBookError),
	)
}

func isClientSafeBookError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, "invalid "),
		strings.Contains(message, "must be "),
		strings.Contains(message, "cannot "),
		strings.Contains(message, "should "),
		strings.Contains(message, "section"),
		strings.Contains(message, "field"),
		strings.Contains(message, "image"),
		strings.Contains(message, "pdf"),
		strings.Contains(message, "submission"),
		strings.Contains(message, "version"),
		strings.Contains(message, "book"),
		strings.Contains(message, "email"),
		strings.Contains(message, "multipart"):
		return true
	default:
		return false
	}
}

func bookPathInt(c *gin.Context, key string) (int, bool) {
	value, err := strconv.Atoi(c.Param(key))
	if err != nil {
		apiresponse.WritePathParamError(c, key)
		return 0, false
	}
	return value, true
}

func bookParseQueryInt(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return parsed
}

func bookAuthUserIDFromContext(c *gin.Context) *int {
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

func sanitizeBookContentDispositionFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

func bookWriteBinary(c *gin.Context, resp *BookPDFContent) {
	if resp == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName := sanitizeBookContentDispositionFilename(resp.FileName); fileName != "" {
		c.Header("Content-Disposition", "inline; filename="+strconv.Quote(fileName))
	}

	c.Data(http.StatusOK, contentType, resp.Content)
}

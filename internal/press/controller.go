package press

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"

	"github.com/gin-gonic/gin"
)

type PressController struct {
	PressService PressServicePort
}

// ListPressEntries retrieves all press entries with optional filtering
func (pc *PressController) ListPressEntries(c *gin.Context) {
	filter := ListPressFilter{
		Status:     strings.TrimSpace(c.DefaultQuery("status", "")),
		Visibility: strings.TrimSpace(c.DefaultQuery("visibility", "")),
		SearchTerm: strings.TrimSpace(c.DefaultQuery("search", "")),
		SortBy:     strings.TrimSpace(c.DefaultQuery("sort_by", "release_date")),
		SortOrder:  strings.TrimSpace(c.DefaultQuery("sort_order", "desc")),
	}

	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")

	pageInt, err := strconv.Atoi(page)
	if err != nil || pageInt < 1 {
		pageInt = 1
	}

	pageSizeInt, err := strconv.Atoi(pageSize)
	if err != nil || pageSizeInt < 1 || pageSizeInt > 100 {
		pageSizeInt = 20
	}

	filter.Page = pageInt
	filter.PageSize = pageSizeInt

	resp, err := pc.PressService.ListPressEntries(filter)
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPressEntry retrieves a single press entry by ID
func (pc *PressController) GetPressEntry(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := pc.PressService.GetPressEntry(id)
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPressMediaContent retrieves the actual media file content
func (pc *PressController) GetPressMediaContent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}
	mediaID, ok := pathInt(c, "mediaId")
	if !ok {
		return
	}

	resp, err := pc.PressService.GetPressMediaContent(id, mediaID)
	if err != nil {
		writePressError(c, err)
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

// CreatePressEntry creates a new press entry
func (pc *PressController) CreatePressEntry(c *gin.Context) {
	req, ok := bindSavePressEntryRequest(c)
	if !ok {
		return
	}

	resp, err := pc.PressService.CreatePressEntry(req, authUserID(c))
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Press entry created successfully", "entry": resp})
}

// UpdatePressEntry updates an existing press entry
func (pc *PressController) UpdatePressEntry(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindSavePressEntryRequest(c)
	if !ok {
		return
	}

	resp, err := pc.PressService.UpdatePressEntry(id, req, authUserID(c))
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Press entry updated successfully", "entry": resp})
}

// DeletePressEntry deletes a press entry
func (pc *PressController) DeletePressEntry(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := pc.PressService.DeletePressEntry(id); err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Press entry deleted successfully"})
}

// AddPressMedia adds media files to a press entry
func (pc *PressController) AddPressMedia(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindAddPressMediaRequest(c)
	if !ok {
		return
	}

	resp, err := pc.PressService.AddPressMedia(id, req, authUserID(c))
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Media files added successfully", "result": resp})
}

// UpdatePressMedia updates metadata for a media file
func (pc *PressController) UpdatePressMedia(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}
	mediaID, ok := pathInt(c, "mediaId")
	if !ok {
		return
	}

	req, ok := bindUpdatePressMediaRequest(c)
	if !ok {
		return
	}

	resp, err := pc.PressService.UpdatePressMedia(id, mediaID, req)
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Media updated successfully", "media": resp})
}

// ReorderPressMedia reorders media files in a press entry
func (pc *PressController) ReorderPressMedia(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindReorderPressMediaRequest(c)
	if !ok {
		return
	}

	resp, err := pc.PressService.ReorderPressMedia(id, req.MediaIDs)
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Media reordered successfully", "result": resp})
}

// DeletePressMedia deletes media files from a press entry
func (pc *PressController) DeletePressMedia(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	req, ok := bindDeletePressMediaRequest(c)
	if !ok {
		return
	}

	resp, err := pc.PressService.DeletePressMedia(id, req.MediaIDs)
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Media deleted successfully", "result": resp})
}

// Helper functions

func pathInt(c *gin.Context, param string) (int, bool) {
	value := strings.TrimSpace(c.Param(param))
	if value == "" {
		apiresponse.WriteValidationError(c, param+" is required")
		return 0, false
	}

	id, err := strconv.Atoi(value)
	if err != nil {
		apiresponse.WriteValidationError(c, param+" must be a valid integer")
		return 0, false
	}

	if id <= 0 {
		apiresponse.WriteValidationError(c, param+" must be positive")
		return 0, false
	}

	return id, true
}

func authUserID(c *gin.Context) *int {
	val, exists := c.Get("userID")
	if !exists {
		return nil
	}

	userID, ok := val.(int)
	if !ok {
		return nil
	}

	return &userID
}

func sanitizeContentDispositionFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	// Basic sanitization - remove path separators
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")
	return filename
}

func writePressError(c *gin.Context, err error) {
	if err == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	switch {
	case errors.Is(err, ErrStoreUnavailable):
		apiresponse.WriteInternalError(c)
	case errors.Is(err, ErrPressEntryNotFound):
		apiresponse.WriteNotFoundError(c, "Press entry not found")
	case errors.Is(err, ErrPressMediaNotFound):
		apiresponse.WriteNotFoundError(c, "Press media not found")
	case errors.Is(err, ErrMediaBucketNotConfigured):
		apiresponse.WriteInternalError(c)
	default:
		apiresponse.WriteValidationError(c, err.Error())
	}
}

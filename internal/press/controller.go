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

func (pc *PressController) ListPressEntries(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	filter := ListPressFilter{
		Status:     strings.TrimSpace(c.DefaultQuery("status", "")),
		Visibility: strings.TrimSpace(c.DefaultQuery("visibility", "")),
		SearchTerm: strings.TrimSpace(c.DefaultQuery("search", "")),
		SortBy:     strings.TrimSpace(c.DefaultQuery("sort_by", "release_date")),
		SortOrder:  strings.TrimSpace(c.DefaultQuery("sort_order", "desc")),
		Page:       queryInt(c, "page", 1, 1, 0),
		PageSize:   queryInt(c, "page_size", 20, 1, 100),
	}

	resp, err := pc.PressService.ListPressEntries(filter)
	if err != nil {
		writePressError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (pc *PressController) GetPressEntry(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

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

func (pc *PressController) GetPressMediaContent(c *gin.Context) {
	if pc.PressService == nil {
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

func (pc *PressController) CreatePressEntry(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

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

func (pc *PressController) UpdatePressEntry(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

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

func (pc *PressController) DeletePressEntry(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

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

func (pc *PressController) AddPressMedia(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

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

func (pc *PressController) UpdatePressMedia(c *gin.Context) {
	if pc.PressService == nil {
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

func (pc *PressController) ReorderPressMedia(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

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

func (pc *PressController) DeletePressMedia(c *gin.Context) {
	if pc.PressService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

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
		apiresponse.WriteValidationError(c, param+" is required")
		return 0, false
	}

	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		apiresponse.WriteValidationError(c, param+" must be a positive integer")
		return 0, false
	}

	return id, true
}

func authUserID(c *gin.Context) *int {
	for _, key := range []string{"userID", "user_id", "userId"} {
		val, exists := c.Get(key)
		if !exists {
			continue
		}
		switch v := val.(type) {
		case int:
			return &v
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

func writePressError(c *gin.Context, err error) {
	if err == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	switch {
	case errors.Is(err, ErrStoreUnavailable), errors.Is(err, ErrMediaBucketNotConfigured):
		apiresponse.WriteInternalError(c)
	case errors.Is(err, ErrPressEntryNotFound):
		writePressNotFound(c, "Press entry not found")
	case errors.Is(err, ErrPressMediaNotFound):
		writePressNotFound(c, "Press media not found")
	default:
		apiresponse.WriteValidationError(c, err.Error())
	}
}

func writePressNotFound(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": message})
}
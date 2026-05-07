package events

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nordikcsaaapi/internal/apiresponse"

	"github.com/gin-gonic/gin"
)

type EventController struct {
	EventService EventServicePort
}

func (ec *EventController) ListEvents(c *gin.Context) {
	filter, err := listEventsFilterFromQuery(c)
	if err != nil {
		apiresponse.WriteValidationError(c, err.Error())
		return
	}

	resp, err := ec.EventService.ListEvents(filter)
	if err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (ec *EventController) GetEvent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := ec.EventService.GetEvent(id)
	if err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (ec *EventController) ListSavedLocations(c *gin.Context) {
	resp, err := ec.EventService.ListSavedLocations()
	if err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (ec *EventController) ListGalleries(c *gin.Context) {
	resp, err := ec.EventService.ListGalleries()
	if err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (ec *EventController) CreateEvent(c *gin.Context) {
	var req SaveEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	event, err := ec.EventService.CreateEvent(req)
	if err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Event created successfully",
		"event":   event,
	})
}

func (ec *EventController) UpdateEvent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	var req SaveEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteBindingError(c, err, req)
		return
	}

	event, err := ec.EventService.UpdateEvent(id, req)
	if err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Event updated successfully",
		"event":   event,
	})
}

func (ec *EventController) DeleteEvent(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	if err := ec.EventService.DeleteEvent(id); err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event deleted successfully"})
}

func (ec *EventController) DeleteEventDocument(c *gin.Context) {
	id, mediaID, ok := pathEventAndMediaIDs(c)
	if !ok {
		return
	}

	if err := ec.EventService.DeleteEventDocument(id, mediaID); err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document deleted successfully"})
}

func (ec *EventController) DeleteAllEventDocuments(c *gin.Context) {
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	resp, err := ec.EventService.DeleteAllEventDocuments(id)
	if err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Documents deleted successfully",
		"deletedCount": resp.DeletedCount,
	})
}

func (ec *EventController) DeleteEventPhoto(c *gin.Context) {
	id, mediaID, ok := pathEventAndMediaIDs(c)
	if !ok {
		return
	}

	if err := ec.EventService.DeleteEventPhoto(id, mediaID); err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Photo deleted successfully"})
}

func writeEventError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrStoreUnavailable), errors.Is(err, ErrMediaBucketNotConfigured):
		apiresponse.WriteServiceUnavailable(c, "Event service is temporarily unavailable")
	case errors.Is(err, ErrEventNotFound), errors.Is(err, ErrEventMediaNotFound):
		apiresponse.WriteNotFound(c, err.Error())
	default:
		apiresponse.WriteValidationError(c, err.Error())
	}
}

func pathEventAndMediaIDs(c *gin.Context) (int, int, bool) {
	id, ok := pathInt(c, "id")
	if !ok {
		return 0, 0, false
	}
	mediaID, ok := pathInt(c, "mediaId")
	if !ok {
		return 0, 0, false
	}
	return id, mediaID, true
}

func pathInt(c *gin.Context, key string) (int, bool) {
	value, err := strconv.Atoi(c.Param(key))
	if err != nil {
		apiresponse.WritePathParamError(c, key)
		return 0, false
	}
	return value, true
}

func listEventsFilterFromQuery(c *gin.Context) (ListEventsFilter, error) {
	filter := ListEventsFilter{
		Page:       parseQueryInt(c.Query("page")),
		PageSize:   parseQueryInt(c.Query("page_size")),
		SearchTerm: c.Query("search"),
		DateRange:  c.Query("date_range"),
		SortBy:     c.DefaultQuery("sort_by", "start_at"),
		SortOrder:  c.DefaultQuery("sort_order", "desc"),
	}

	statuses := append([]string{}, c.QueryArray("status")...)
	statuses = append(statuses, c.QueryArray("statuses")...)
	filter.Statuses = splitQueryValues(statuses...)

	startDate, err := parseOptionalDate(c.Query("start_date"))
	if err != nil {
		return filter, err
	}
	endDate, err := parseOptionalDate(c.Query("end_date"))
	if err != nil {
		return filter, err
	}
	filter.StartDate = startDate
	filter.EndDate = endDate

	return filter, nil
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

func parseOptionalDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}

	return nil, errors.New("invalid date format; use RFC3339 or YYYY-MM-DD")
}

func splitQueryValues(values ...string) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
	}
	return parts
}

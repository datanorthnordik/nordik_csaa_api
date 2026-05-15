package events

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

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

func (ec *EventController) GetEventMediaContent(c *gin.Context) {
	id, mediaID, ok := pathEventAndMediaIDs(c)
	if !ok {
		return
	}

	resp, err := ec.EventService.GetEventMediaContent(id, mediaID)
	if err != nil {
		writeEventError(c, err)
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
	req, ok := bindSaveEventRequest(c)
	if !ok {
		return
	}
	req.CreatedBy = authUserIDFromContext(c)

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

	req, ok := bindSaveEventRequest(c)
	if !ok {
		return
	}
	req.CreatedBy = nil

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
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	storageURL, ok := storageURLFromQuery(c)
	if !ok {
		return
	}

	if err := ec.EventService.DeleteEventDocument(id, storageURL); err != nil {
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

	var req DeleteEventMediaBatchRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			apiresponse.WriteBindingError(c, err, req)
			return
		}
	}

	resp, err := ec.EventService.DeleteAllEventDocuments(id, req.StorageURLs)
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
	id, ok := pathInt(c, "id")
	if !ok {
		return
	}

	storageURL, ok := storageURLFromQuery(c)
	if !ok {
		return
	}

	if err := ec.EventService.DeleteEventPhoto(id, storageURL); err != nil {
		writeEventError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Photo deleted successfully"})
}

func writeEventError(c *gin.Context, err error) {
	if message, ok := eventValidationMessageFromStoreError(err); ok {
		apiresponse.WriteValidationError(c, message)
		return
	}

	httpapi.HandleError(c, "events", err,
		httpapi.ServiceUnavailableRule("Event service is temporarily unavailable", ErrStoreUnavailable, ErrMediaBucketNotConfigured),
		httpapi.NotFoundRule(ErrEventNotFound, ErrEventMediaNotFound),
		httpapi.ConflictRule("Unable to save event because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeEventError),
	)
}

func eventValidationMessageFromStoreError(err error) (string, bool) {
	if err == nil {
		return "", false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	if mapped := mapEventConstraintMessage(message); mapped != "" {
		return mapped, true
	}

	switch {
	case strings.Contains(message, "sqlstate 23514"),
		strings.Contains(message, "violates check constraint"):
		return "event payload violates a database constraint", true
	case strings.Contains(message, "sqlstate 23503"),
		strings.Contains(message, "violates foreign key constraint"):
		return "event payload references a related record that does not exist", true
	case strings.Contains(message, "sqlstate 23502"),
		strings.Contains(message, "violates not-null constraint"):
		return "event payload is missing a required field", true
	default:
		return "", false
	}
}

func mapEventConstraintMessage(message string) string {
	switch {
	case strings.Contains(message, "chk_events_title_not_blank"):
		return "title is required"
	case strings.Contains(message, "chk_events_categories_required"):
		return "at least one category is required"
	case strings.Contains(message, "chk_events_event_type"):
		return "invalid event_type"
	case strings.Contains(message, "chk_events_end_after_start"):
		return "end_at must be on or after start_at"
	case strings.Contains(message, "chk_events_event_type_dates"):
		return "event dates do not match event_type"
	case strings.Contains(message, "chk_events_privacy_type"):
		return "invalid privacy_type"
	case strings.Contains(message, "chk_events_private_audiences"):
		return "private_audiences must match privacy_type"
	case strings.Contains(message, "chk_events_location_mode"):
		return "invalid location_mode"
	case strings.Contains(message, "chk_events_location_address"):
		return "address details are required when location_mode is address"
	case strings.Contains(message, "chk_events_review_request_emails"):
		return "review_email_list must match request_review"
	case strings.Contains(message, "chk_events_published_review"):
		return "request_review cannot be true when published is true"
	case strings.Contains(message, "chk_events_registration"):
		return "registration_start_at and registration_end_at are required when registration_enabled is true"
	case strings.Contains(message, "chk_events_recurrence_type"):
		return "invalid recurrence_type"
	case strings.Contains(message, "chk_events_recurrence_frequency"):
		return "invalid recurrence_frequency"
	case strings.Contains(message, "chk_events_recurrence_interval"):
		return "recurrence_interval must be greater than zero"
	case strings.Contains(message, "chk_events_repeat_definition"):
		return "recurrence_type is required when repeat_enabled is true"
	case strings.Contains(message, "chk_events_recurring_requires_frequency"):
		return "recurrence_frequency is required when recurrence_type is recurring"
	case strings.Contains(message, "chk_events_scheduled_has_no_frequency"):
		return "recurrence_frequency must be empty when recurrence_type is scheduled"
	case strings.Contains(message, "chk_events_recurrence_rule_json"):
		return "recurrence_rule must be valid json"
	case strings.Contains(message, "fk_events_address"):
		return "address not found"
	case strings.Contains(message, "fk_events_gallery"):
		return "gallery not found"
	case strings.Contains(message, "fk_events_created_by"):
		return "created_by is invalid"
	default:
		return ""
	}
}

func isClientSafeEventError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " are required"),
		strings.Contains(message, " must be "),
		strings.Contains(message, " must be on or after "),
		strings.Contains(message, " must be omitted "),
		strings.Contains(message, " must be empty "),
		strings.Contains(message, "invalid "),
		strings.Contains(message, "missing both uploaded file and file_url"),
		strings.Contains(message, "use multipart/form-data"),
		strings.Contains(message, "at least one "),
		strings.Contains(message, "cannot be true when "),
		strings.Contains(message, "must be valid json"),
		strings.Contains(message, "address not found"):
		return true
	default:
		return false
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

func storageURLFromQuery(c *gin.Context) (string, bool) {
	value := strings.TrimSpace(c.Query("storage_url"))
	if value == "" {
		apiresponse.WriteValidationError(c, "storage_url is required")
		return "", false
	}
	return value, true
}

func pathInt(c *gin.Context, key string) (int, bool) {
	value, err := strconv.Atoi(c.Param(key))
	if err != nil {
		apiresponse.WritePathParamError(c, key)
		return 0, false
	}
	return value, true
}

func authUserIDFromContext(c *gin.Context) *int {
	if c == nil {
		return nil
	}

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

func sanitizeContentDispositionFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

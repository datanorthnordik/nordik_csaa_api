package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable         = errors.New("event store unavailable")
	ErrEventNotFound            = errors.New("event not found")
	ErrEventMediaNotFound       = errors.New("event media not found")
	ErrMediaBucketNotConfigured = errors.New("drive bucket is not configured")
)

var (
	nowFunc               = time.Now
	uploadBase64ToGCSHook = func(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
		return util.UploadBase64ToGCS(base64Data, bucketName, objectName, contentType)
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error {
		return util.DeleteGCSObject(bucketName, objectName)
	}
)

type EventService struct {
	DB         *gorm.DB
	BucketName string
}

func (s *EventService) ListEvents(filter ListEventsFilter) (*EventListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalized, err := normalizeListEventsFilter(filter)
	if err != nil {
		return nil, err
	}

	scopes, err := buildListEventScopes(normalized, nowFunc())
	if err != nil {
		return nil, err
	}

	baseQuery := s.DB.Model(&Event{})
	for _, scope := range scopes {
		baseQuery = scope(baseQuery)
	}

	var totalItems int64
	if err := baseQuery.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	offset := (normalized.Page - 1) * normalized.PageSize
	var rows []Event
	if err := baseQuery.
		Order(buildEventSortClause(normalized.SortBy, normalized.SortOrder)).
		Offset(offset).
		Limit(normalized.PageSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]EventListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, EventListItem{
			ID:         row.ID,
			Title:      row.Title,
			Categories: []string(row.Categories),
			Status:     eventStatus(row.Published),
			Published:  row.Published,
			EventType:  row.EventType,
			StartAt:    row.StartAt,
			EndAt:      row.EndAt,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		})
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(normalized.PageSize) - 1) / int64(normalized.PageSize))
	}

	return &EventListResponse{
		Items: items,
		Pagination: EventListPageMeta{
			Page:       normalized.Page,
			PageSize:   normalized.PageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    normalized.Page < totalPages,
			HasPrev:    normalized.Page > 1,
		},
		Applied: normalized,
	}, nil
}

func (s *EventService) GetEvent(id int) (*EventDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var event Event
	if err := s.DB.First(&event, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEventNotFound
		}
		return nil, err
	}

	var address *Address
	if event.AddressID != nil {
		var row Address
		if err := s.DB.First(&row, *event.AddressID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrEventNotFound
			}
			return nil, err
		}
		address = &row
	}

	var media []EventMedia
	if err := s.DB.Where("event_id = ?", event.ID).Order("sort_order ASC").Order("id ASC").Find(&media).Error; err != nil {
		return nil, err
	}

	var occurrences []EventOccurrence
	if err := s.DB.Where("event_id = ?", event.ID).Order("occurrence_start_at ASC").Order("id ASC").Find(&occurrences).Error; err != nil {
		return nil, err
	}

	var displayImage *EventMedia
	attachments := make([]EventMedia, 0)
	for _, item := range media {
		switch item.MediaRole {
		case MediaRoleDisplayImage:
			copyItem := item
			displayImage = &copyItem
		case MediaRoleAttachment:
			attachments = append(attachments, item)
		}
	}

	return &EventDetailResponse{
		ID:                          event.ID,
		Title:                       event.Title,
		ShowTitle:                   event.ShowTitle,
		Categories:                  []string(event.Categories),
		EventType:                   event.EventType,
		StartAt:                     event.StartAt,
		EndAt:                       event.EndAt,
		PrivacyType:                 event.PrivacyType,
		PrivateAudiences:            []string(event.PrivateAudiences),
		Published:                   event.Published,
		RequestReview:               event.RequestReview,
		ReviewEmailList:             []string(event.ReviewEmailList),
		Teaser:                      event.Teaser,
		DescriptionHTML:             event.DescriptionHTML,
		ContactName:                 event.ContactName,
		ContactEmail:                event.ContactEmail,
		ContactPhone:                event.ContactPhone,
		ContactExt:                  event.ContactExt,
		ContactFax:                  event.ContactFax,
		LocationMode:                event.LocationMode,
		Address:                     address,
		ShowDisplayImageWhenViewing: event.ShowDisplayImageWhenViewing,
		GalleryID:                   event.GalleryID,
		RegistrationEnabled:         event.RegistrationEnabled,
		RegistrationStartAt:         event.RegistrationStartAt,
		RegistrationEndAt:           event.RegistrationEndAt,
		RegistrationURL:             event.RegistrationURL,
		RepeatEnabled:               event.RepeatEnabled,
		RecurrenceType:              event.RecurrenceType,
		RecurrenceFrequency:         event.RecurrenceFrequency,
		RecurrenceInterval:          event.RecurrenceInterval,
		RecurrenceUntil:             event.RecurrenceUntil,
		RecurrenceRule:              event.RecurrenceRule,
		Occurrences:                 occurrences,
		DisplayImage:                displayImage,
		Attachments:                 attachments,
		CreatedBy:                   event.CreatedBy,
		CreatedAt:                   event.CreatedAt,
		UpdatedAt:                   event.UpdatedAt,
	}, nil
}

func (s *EventService) ListSavedLocations() (*SavedLocationListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var rows []Address
	if err := s.DB.
		Where("is_saved = ?", true).
		Order("LOWER(name) ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return &SavedLocationListResponse{Items: rows}, nil
}

func (s *EventService) ListGalleries() (*GalleryListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var rows []Gallery
	if err := s.DB.
		Order("LOWER(name) ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return &GalleryListResponse{Items: rows}, nil
}

func (s *EventService) CreateEvent(req SaveEventRequest) (*EventMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalized, err := normalizeSaveEventRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	uploadedObjects := make([]string, 0)
	defer rollbackOnPanic(tx)

	addressID, err := s.resolveAddress(tx, normalized.LocationMode, normalized.Address)
	if err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	event := buildEventModel(normalized)
	event.AddressID = addressID

	if err := tx.Create(&event).Error; err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.saveDisplayImage(tx, event.ID, normalized.DisplayImage, &uploadedObjects); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.saveAttachments(tx, event.ID, normalized.Attachments, &uploadedObjects); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.replaceOccurrences(tx, event.ID, normalized.Occurrences); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &EventMutationResponse{
		ID:        event.ID,
		Title:     event.Title,
		Published: event.Published,
	}, nil
}

func (s *EventService) UpdateEvent(id int, req SaveEventRequest) (*EventMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalized, err := normalizeSaveEventRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	uploadedObjects := make([]string, 0)
	oldObjectsToDelete := make([]EventMedia, 0)
	defer rollbackOnPanic(tx)

	var event Event
	if err := tx.First(&event, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEventNotFound
		}
		return nil, err
	}

	addressID, err := s.resolveAddress(tx, normalized.LocationMode, normalized.Address)
	if err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	applyEventRequest(&event, normalized)
	event.AddressID = addressID

	if err := tx.Save(&event).Error; err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if normalized.DisplayImage != nil {
		var existingDisplay EventMedia
		if err := tx.Where("event_id = ? AND media_role = ?", event.ID, MediaRoleDisplayImage).First(&existingDisplay).Error; err == nil {
			if err := tx.Delete(&existingDisplay).Error; err != nil {
				tx.Rollback()
				s.cleanupObjects(uploadedObjects)
				return nil, err
			}
			oldObjectsToDelete = append(oldObjectsToDelete, existingDisplay)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}

		if err := s.saveDisplayImage(tx, event.ID, normalized.DisplayImage, &uploadedObjects); err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := s.saveAttachments(tx, event.ID, normalized.Attachments, &uploadedObjects); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.replaceOccurrences(tx, event.ID, normalized.Occurrences); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.cleanupMediaObjects(oldObjectsToDelete); err != nil {
		return nil, err
	}

	return &EventMutationResponse{
		ID:        event.ID,
		Title:     event.Title,
		Published: event.Published,
	}, nil
}

func (s *EventService) DeleteEvent(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackOnPanic(tx)

	var media []EventMedia
	if err := tx.Where("event_id = ?", id).Find(&media).Error; err != nil {
		tx.Rollback()
		return err
	}

	result := tx.Delete(&Event{}, id)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return ErrEventNotFound
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return s.cleanupMediaObjects(media)
}

func (s *EventService) DeleteEventDocument(id int, mediaID int) error {
	return s.deleteEventMedia(id, mediaID, MediaRoleAttachment)
}

func (s *EventService) DeleteAllEventDocuments(id int) (*DeleteAllDocumentsResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	var media []EventMedia
	if err := tx.Where("event_id = ? AND media_role = ?", id, MediaRoleAttachment).Find(&media).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Where("event_id = ? AND media_role = ?", id, MediaRoleAttachment).Delete(&EventMedia{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	if err := s.cleanupMediaObjects(media); err != nil {
		return nil, err
	}

	return &DeleteAllDocumentsResponse{DeletedCount: len(media)}, nil
}

func (s *EventService) DeleteEventPhoto(id int, mediaID int) error {
	return s.deleteEventMedia(id, mediaID, MediaRoleDisplayImage)
}

func (s *EventService) deleteEventMedia(eventID int, mediaID int, allowedRole string) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackOnPanic(tx)

	var media EventMedia
	if err := tx.Where("event_id = ? AND id = ?", eventID, mediaID).First(&media).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEventMediaNotFound
		}
		return err
	}

	if media.MediaRole != allowedRole {
		tx.Rollback()
		return ErrEventMediaNotFound
	}

	if err := tx.Delete(&media).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return s.cleanupMediaObjects([]EventMedia{media})
}

func normalizeSaveEventRequest(req SaveEventRequest) (SaveEventRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Teaser = strings.TrimSpace(req.Teaser)
	req.EventType = strings.TrimSpace(req.EventType)
	req.PrivacyType = strings.TrimSpace(req.PrivacyType)
	req.LocationMode = strings.TrimSpace(req.LocationMode)
	req.RegistrationURL = strings.TrimSpace(req.RegistrationURL)
	req.RecurrenceType = strings.TrimSpace(req.RecurrenceType)
	req.RecurrenceFrequency = strings.TrimSpace(req.RecurrenceFrequency)
	req.Categories = sanitizeStringSlice(req.Categories)
	req.PrivateAudiences = sanitizeStringSlice(req.PrivateAudiences)
	req.ReviewEmailList = sanitizeStringSlice(req.ReviewEmailList)
	req.Attachments = sanitizeUploadInputs(req.Attachments)
	if req.DisplayImage != nil {
		cleaned := sanitizeUploadInput(*req.DisplayImage)
		req.DisplayImage = &cleaned
	}
	if req.Address != nil {
		req.Address.Name = strings.TrimSpace(req.Address.Name)
		req.Address.AddressLine1 = strings.TrimSpace(req.Address.AddressLine1)
		req.Address.AddressLine2 = strings.TrimSpace(req.Address.AddressLine2)
		req.Address.City = strings.TrimSpace(req.Address.City)
		req.Address.ProvinceState = strings.TrimSpace(req.Address.ProvinceState)
		req.Address.PostalCode = strings.TrimSpace(req.Address.PostalCode)
		req.Address.Country = strings.TrimSpace(req.Address.Country)
	}

	if req.PrivacyType == "" {
		req.PrivacyType = "public"
	}
	if req.LocationMode == "" {
		req.LocationMode = "none"
	}
	if req.RecurrenceInterval == 0 {
		req.RecurrenceInterval = 1
	}

	if req.Title == "" {
		return req, errors.New("title is required")
	}
	if req.Teaser == "" {
		return req, errors.New("teaser is required")
	}
	if len(req.Categories) == 0 {
		return req, errors.New("at least one category is required")
	}
	if !isAllowed(req.EventType, "single_day_all_day", "single_day_partial", "multi_day_all_day", "multi_day_partial") {
		return req, errors.New("invalid event_type")
	}
	if req.EndAt != nil && req.EndAt.Before(req.StartAt) {
		return req, errors.New("end_at must be on or after start_at")
	}
	if err := validateEventTypeTimeRules(req); err != nil {
		return req, err
	}
	if !isAllowed(req.PrivacyType, "public", "private") {
		return req, errors.New("invalid privacy_type")
	}
	if req.PrivacyType == "private" && len(req.PrivateAudiences) == 0 {
		return req, errors.New("private_audiences are required when privacy_type is private")
	}
	if !isAllowed(req.LocationMode, "none", "to_be_determined", "address") {
		return req, errors.New("invalid location_mode")
	}
	if req.LocationMode == "address" && req.Address == nil {
		return req, errors.New("address details are required when location_mode is address")
	}
	if req.Published && req.RequestReview {
		return req, errors.New("request_review cannot be true when published is true")
	}
	if !req.RequestReview && len(req.ReviewEmailList) > 0 {
		return req, errors.New("review_email_list must be empty when request_review is false")
	}
	if req.RequestReview && len(req.ReviewEmailList) == 0 {
		return req, errors.New("review_email_list is required when request_review is true")
	}
	if req.RegistrationEnabled {
		if req.RegistrationStartAt == nil || req.RegistrationEndAt == nil {
			return req, errors.New("registration_start_at and registration_end_at are required when registration_enabled is true")
		}
		if req.RegistrationEndAt.Before(*req.RegistrationStartAt) {
			return req, errors.New("registration_end_at must be on or after registration_start_at")
		}
		if req.RegistrationURL == "" {
			return req, errors.New("registration_url is required when registration_enabled is true")
		}
	}
	if req.RepeatEnabled {
		if !isAllowed(req.RecurrenceType, "scheduled", "recurring") {
			return req, errors.New("recurrence_type is required when repeat_enabled is true")
		}
		if req.RecurrenceType == "recurring" && !isAllowed(req.RecurrenceFrequency, "daily", "weekly", "monthly", "yearly") {
			return req, errors.New("invalid recurrence_frequency")
		}
		if req.RecurrenceType == "scheduled" {
			req.RecurrenceFrequency = ""
		}
		if req.RecurrenceInterval < 1 {
			return req, errors.New("recurrence_interval must be greater than zero")
		}
	} else {
		req.RecurrenceType = ""
		req.RecurrenceFrequency = ""
		req.RecurrenceInterval = 1
		req.RecurrenceUntil = nil
		req.RecurrenceRule = nil
	}

	for _, occurrence := range req.Occurrences {
		if occurrence.OccurrenceEndAt != nil && occurrence.OccurrenceEndAt.Before(occurrence.OccurrenceStartAt) {
			return req, errors.New("occurrence_end_at must be on or after occurrence_start_at")
		}
		if occurrence.OccurrenceKind == "" {
			occurrence.OccurrenceKind = "scheduled"
		}
		if !isAllowed(strings.TrimSpace(occurrence.OccurrenceKind), "scheduled", "generated", "exception") {
			return req, errors.New("invalid occurrence_kind")
		}
	}

	if len(req.RecurrenceRule) > 0 && !json.Valid(req.RecurrenceRule) {
		return req, errors.New("recurrence_rule must be valid json")
	}

	return req, nil
}

func buildEventModel(req SaveEventRequest) Event {
	return Event{
		Title:                       req.Title,
		ShowTitle:                   req.ShowTitle,
		Categories:                  pq.StringArray(req.Categories),
		EventType:                   req.EventType,
		StartAt:                     req.StartAt,
		EndAt:                       req.EndAt,
		PrivacyType:                 req.PrivacyType,
		PrivateAudiences:            pq.StringArray(req.PrivateAudiences),
		Published:                   req.Published,
		RequestReview:               req.RequestReview,
		ReviewEmailList:             pq.StringArray(req.ReviewEmailList),
		Teaser:                      req.Teaser,
		DescriptionHTML:             req.DescriptionHTML,
		ContactName:                 req.ContactName,
		ContactEmail:                req.ContactEmail,
		ContactPhone:                req.ContactPhone,
		ContactExt:                  req.ContactExt,
		ContactFax:                  req.ContactFax,
		LocationMode:                req.LocationMode,
		ShowDisplayImageWhenViewing: req.ShowDisplayImageWhenViewing,
		GalleryID:                   req.GalleryID,
		RegistrationEnabled:         req.RegistrationEnabled,
		RegistrationStartAt:         req.RegistrationStartAt,
		RegistrationEndAt:           req.RegistrationEndAt,
		RegistrationURL:             req.RegistrationURL,
		RepeatEnabled:               req.RepeatEnabled,
		RecurrenceType:              stringPtrOrNil(req.RecurrenceType),
		RecurrenceFrequency:         stringPtrOrNil(req.RecurrenceFrequency),
		RecurrenceInterval:          req.RecurrenceInterval,
		RecurrenceUntil:             req.RecurrenceUntil,
		RecurrenceRule:              JSONRawMessage(req.RecurrenceRule),
		CreatedBy:                   req.CreatedBy,
	}
}

func applyEventRequest(event *Event, req SaveEventRequest) {
	updated := buildEventModel(req)
	event.Title = updated.Title
	event.ShowTitle = updated.ShowTitle
	event.Categories = updated.Categories
	event.EventType = updated.EventType
	event.StartAt = updated.StartAt
	event.EndAt = updated.EndAt
	event.PrivacyType = updated.PrivacyType
	event.PrivateAudiences = updated.PrivateAudiences
	event.Published = updated.Published
	event.RequestReview = updated.RequestReview
	event.ReviewEmailList = updated.ReviewEmailList
	event.Teaser = updated.Teaser
	event.DescriptionHTML = updated.DescriptionHTML
	event.ContactName = updated.ContactName
	event.ContactEmail = updated.ContactEmail
	event.ContactPhone = updated.ContactPhone
	event.ContactExt = updated.ContactExt
	event.ContactFax = updated.ContactFax
	event.LocationMode = updated.LocationMode
	event.ShowDisplayImageWhenViewing = updated.ShowDisplayImageWhenViewing
	event.GalleryID = updated.GalleryID
	event.RegistrationEnabled = updated.RegistrationEnabled
	event.RegistrationStartAt = updated.RegistrationStartAt
	event.RegistrationEndAt = updated.RegistrationEndAt
	event.RegistrationURL = updated.RegistrationURL
	event.RepeatEnabled = updated.RepeatEnabled
	event.RecurrenceType = updated.RecurrenceType
	event.RecurrenceFrequency = updated.RecurrenceFrequency
	event.RecurrenceInterval = updated.RecurrenceInterval
	event.RecurrenceUntil = updated.RecurrenceUntil
	event.RecurrenceRule = updated.RecurrenceRule
	event.CreatedBy = updated.CreatedBy
}

func (s *EventService) resolveAddress(tx *gorm.DB, locationMode string, input *EventAddressInput) (*int, error) {
	if locationMode != "address" || input == nil {
		return nil, nil
	}

	if input.ID != nil {
		var address Address
		if err := tx.First(&address, *input.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("address not found")
			}
			return nil, err
		}
		return input.ID, nil
	}

	address := Address{
		Name:          input.Name,
		AddressLine1:  input.AddressLine1,
		AddressLine2:  input.AddressLine2,
		City:          input.City,
		ProvinceState: input.ProvinceState,
		PostalCode:    input.PostalCode,
		Country:       input.Country,
		IsSaved:       input.IsSaved,
	}
	if err := tx.Create(&address).Error; err != nil {
		return nil, err
	}
	return &address.ID, nil
}

func (s *EventService) saveDisplayImage(tx *gorm.DB, eventID int, input *EventUploadInput, uploadedObjects *[]string) error {
	if input == nil {
		return nil
	}

	media, uploadedObject, err := s.buildMediaRecord(eventID, MediaRoleDisplayImage, 0, *input)
	if err != nil {
		return err
	}
	if uploadedObject != "" {
		*uploadedObjects = append(*uploadedObjects, uploadedObject)
	}
	return tx.Create(&media).Error
}

func (s *EventService) saveAttachments(tx *gorm.DB, eventID int, inputs []EventUploadInput, uploadedObjects *[]string) error {
	for idx, input := range inputs {
		media, uploadedObject, err := s.buildMediaRecord(eventID, MediaRoleAttachment, idx, input)
		if err != nil {
			return err
		}
		if uploadedObject != "" {
			*uploadedObjects = append(*uploadedObjects, uploadedObject)
		}
		if err := tx.Create(&media).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *EventService) buildMediaRecord(eventID int, role string, idx int, input EventUploadInput) (EventMedia, string, error) {
	media := EventMedia{
		EventID:     eventID,
		MediaRole:   role,
		DisplayName: input.DisplayName,
		MimeType:    input.MimeType,
		SortOrder:   idx,
	}

	if strings.TrimSpace(input.DataBase64) == "" {
		if strings.TrimSpace(input.FileURL) == "" {
			return EventMedia{}, "", fmt.Errorf("%s upload %d is missing both data_base64 and file_url", role, idx+1)
		}
		media.FileURL = strings.TrimSpace(input.FileURL)
		media.GCPObjectKey = strings.TrimSpace(input.ObjectKey)
		if media.GCPObjectKey == "" && strings.TrimSpace(s.BucketName) != "" {
			objectKey, err := util.ExtractObjectPathFromGCSURL(s.BucketName, media.FileURL)
			if err == nil {
				media.GCPObjectKey = objectKey
			}
		}
		return media, "", nil
	}

	if strings.TrimSpace(s.BucketName) == "" {
		return EventMedia{}, "", ErrMediaBucketNotConfigured
	}

	objectName := s.mediaObjectName(eventID, role, idx, input.FileName, input.MimeType)
	fileURL, sizeBytes, err := uploadBase64ToGCSHook(input.DataBase64, s.BucketName, objectName, strings.TrimSpace(input.MimeType))
	if err != nil {
		return EventMedia{}, "", err
	}

	media.FileURL = fileURL
	media.GCPObjectKey = objectName
	media.FileSize = sizeBytes
	return media, objectName, nil
}

func (s *EventService) replaceOccurrences(tx *gorm.DB, eventID int, occurrences []EventOccurrenceInput) error {
	if err := tx.Where("event_id = ?", eventID).Delete(&EventOccurrence{}).Error; err != nil {
		return err
	}

	for _, occurrence := range occurrences {
		kind := strings.TrimSpace(occurrence.OccurrenceKind)
		if kind == "" {
			kind = "scheduled"
		}
		row := EventOccurrence{
			EventID:           eventID,
			OccurrenceStartAt: occurrence.OccurrenceStartAt,
			OccurrenceEndAt:   occurrence.OccurrenceEndAt,
			OccurrenceKind:    kind,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *EventService) mediaObjectName(eventID int, role string, idx int, fileName string, mimeType string) string {
	timestamp := nowFunc().UTC().Format("20060102150405")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = role
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("events/%d/%s_%s_%d_%s%s", eventID, role, timestamp, idx+1, base, ext)
}

func (s *EventService) cleanupMediaObjects(media []EventMedia) error {
	var cleanupErr error
	for _, item := range media {
		if err := s.cleanupSingleMediaObject(item); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func (s *EventService) cleanupSingleMediaObject(media EventMedia) error {
	objectKey := strings.TrimSpace(media.GCPObjectKey)
	if objectKey == "" && strings.TrimSpace(media.FileURL) == "" {
		return nil
	}
	if strings.TrimSpace(s.BucketName) == "" {
		return ErrMediaBucketNotConfigured
	}
	if objectKey == "" {
		var err error
		objectKey, err = util.ExtractObjectPathFromGCSURL(s.BucketName, media.FileURL)
		if err != nil {
			return err
		}
	}
	return deleteGCSObjectHook(s.BucketName, objectKey)
}

func (s *EventService) cleanupObjects(objectNames []string) {
	for _, objectName := range objectNames {
		if strings.TrimSpace(objectName) == "" || strings.TrimSpace(s.BucketName) == "" {
			continue
		}
		_ = deleteGCSObjectHook(s.BucketName, objectName)
	}
}

func sanitizeStringSlice(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func sanitizeUploadInputs(values []EventUploadInput) []EventUploadInput {
	cleaned := make([]EventUploadInput, 0, len(values))
	for _, value := range values {
		cleaned = append(cleaned, sanitizeUploadInput(value))
	}
	return cleaned
}

func sanitizeUploadInput(value EventUploadInput) EventUploadInput {
	value.DisplayName = strings.TrimSpace(value.DisplayName)
	value.FileName = strings.TrimSpace(value.FileName)
	value.MimeType = strings.TrimSpace(value.MimeType)
	value.DataBase64 = strings.TrimSpace(value.DataBase64)
	value.FileURL = strings.TrimSpace(value.FileURL)
	value.ObjectKey = strings.TrimSpace(value.ObjectKey)
	return value
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func normalizeListEventsFilter(filter ListEventsFilter) (ListEventsFilter, error) {
	filter.SearchTerm = strings.TrimSpace(filter.SearchTerm)
	filter.DateRange = strings.TrimSpace(strings.ToLower(filter.DateRange))
	filter.SortBy = strings.TrimSpace(strings.ToLower(filter.SortBy))
	filter.SortOrder = strings.TrimSpace(strings.ToLower(filter.SortOrder))

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 10
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	if filter.DateRange == "" {
		filter.DateRange = EventDateRangeCustom
	}
	if filter.SortBy == "" {
		filter.SortBy = "start_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}
	if !isAllowed(filter.DateRange, EventDateRangeCustom, EventDateRangeNext30Days, EventDateRangeLast30Days, EventDateRangeToday, EventDateRangeThisMonth, EventDateRangeUpcoming) {
		return filter, errors.New("invalid date_range")
	}
	if !isAllowed(filter.SortBy, "title", "start_at", "created_at", "updated_at", "published") {
		return filter, errors.New("invalid sort_by")
	}
	if !isAllowed(filter.SortOrder, "asc", "desc") {
		return filter, errors.New("invalid sort_order")
	}

	statusSet := map[string]struct{}{}
	for _, value := range filter.Statuses {
		status := strings.TrimSpace(strings.ToLower(value))
		if status == "" {
			continue
		}
		if !isAllowed(status, EventStatusPublished, EventStatusDraft) {
			return filter, fmt.Errorf("invalid status %q", status)
		}
		statusSet[status] = struct{}{}
	}

	filter.Statuses = make([]string, 0, len(statusSet))
	for status := range statusSet {
		filter.Statuses = append(filter.Statuses, status)
	}
	sort.Strings(filter.Statuses)

	if filter.StartDate != nil && filter.EndDate != nil && filter.EndDate.Before(*filter.StartDate) {
		return filter, errors.New("end_date must be on or after start_date")
	}

	return filter, nil
}

func buildListEventScopes(filter ListEventsFilter, now time.Time) ([]func(*gorm.DB) *gorm.DB, error) {
	scopes := make([]func(*gorm.DB) *gorm.DB, 0, 4)

	if filter.SearchTerm != "" {
		search := "%" + strings.ToLower(filter.SearchTerm) + "%"
		scopes = append(scopes, func(db *gorm.DB) *gorm.DB {
			return db.Where("LOWER(title) LIKE ?", search)
		})
	}

	if len(filter.Statuses) == 1 {
		isPublished := filter.Statuses[0] == EventStatusPublished
		scopes = append(scopes, func(db *gorm.DB) *gorm.DB {
			return db.Where("published = ?", isPublished)
		})
	}

	rangeStart, rangeEnd, err := resolveEventDateRange(filter, now)
	if err != nil {
		return nil, err
	}
	if rangeStart != nil {
		scopes = append(scopes, func(db *gorm.DB) *gorm.DB {
			return db.Where("start_at >= ?", *rangeStart)
		})
	}
	if rangeEnd != nil {
		scopes = append(scopes, func(db *gorm.DB) *gorm.DB {
			return db.Where("start_at < ?", *rangeEnd)
		})
	}

	return scopes, nil
}

func resolveEventDateRange(filter ListEventsFilter, now time.Time) (*time.Time, *time.Time, error) {
	switch filter.DateRange {
	case EventDateRangeCustom:
		var start *time.Time
		var end *time.Time
		if filter.StartDate != nil {
			s := startOfDay(*filter.StartDate)
			start = &s
		}
		if filter.EndDate != nil {
			e := startOfDay(*filter.EndDate).Add(24 * time.Hour)
			end = &e
		}
		return start, end, nil
	case EventDateRangeNext30Days:
		start := startOfDay(now)
		end := start.AddDate(0, 0, 30)
		return &start, &end, nil
	case EventDateRangeLast30Days:
		end := startOfDay(now).Add(24 * time.Hour)
		start := startOfDay(now).AddDate(0, 0, -30)
		return &start, &end, nil
	case EventDateRangeToday:
		start := startOfDay(now)
		end := start.Add(24 * time.Hour)
		return &start, &end, nil
	case EventDateRangeThisMonth:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0)
		return &start, &end, nil
	case EventDateRangeUpcoming:
		start := now
		return &start, nil, nil
	default:
		return nil, nil, errors.New("invalid date_range")
	}
}

func buildEventSortClause(sortBy string, sortOrder string) string {
	allowedColumns := map[string]string{
		"title":      "title",
		"start_at":   "start_at",
		"created_at": "created_at",
		"updated_at": "updated_at",
		"published":  "published",
	}
	column, ok := allowedColumns[sortBy]
	if !ok {
		column = "start_at"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	return column + " " + strings.ToUpper(sortOrder)
}

func validateEventTypeTimeRules(req SaveEventRequest) error {
	switch req.EventType {
	case "single_day_all_day":
		if req.EndAt != nil {
			return errors.New("end_at must be omitted for single_day_all_day events")
		}
	case "single_day_partial":
		if req.EndAt == nil {
			return errors.New("end_at is required for single_day_partial events")
		}
		if !sameCalendarDate(req.StartAt, *req.EndAt) {
			return errors.New("end_at must be on the same date as start_at for single_day_partial events")
		}
	case "multi_day_all_day":
		if req.EndAt == nil {
			return errors.New("end_at is required for multi_day_all_day events")
		}
		if !isLaterCalendarDate(req.StartAt, *req.EndAt) {
			return errors.New("end_at must be on a later date than start_at for multi_day_all_day events")
		}
	case "multi_day_partial":
		if req.EndAt == nil {
			return errors.New("end_at is required for multi_day_partial events")
		}
		if !isLaterCalendarDate(req.StartAt, *req.EndAt) {
			return errors.New("end_at must be on a later date than start_at for multi_day_partial events")
		}
	}

	return nil
}

func eventStatus(published bool) string {
	if published {
		return EventStatusPublished
	}
	return EventStatusDraft
}

func sameCalendarDate(start time.Time, end time.Time) bool {
	return compareCalendarDate(start, end) == 0
}

func isLaterCalendarDate(start time.Time, end time.Time) bool {
	return compareCalendarDate(start, end) < 0
}

func compareCalendarDate(left time.Time, right time.Time) int {
	right = right.In(left.Location())

	leftYear, leftMonth, leftDay := left.Date()
	rightYear, rightMonth, rightDay := right.Date()

	switch {
	case leftYear < rightYear:
		return -1
	case leftYear > rightYear:
		return 1
	case leftMonth < rightMonth:
		return -1
	case leftMonth > rightMonth:
		return 1
	case leftDay < rightDay:
		return -1
	case leftDay > rightDay:
		return 1
	default:
		return 0
	}
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func isAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func rollbackOnPanic(tx *gorm.DB) {
	if recover() != nil {
		tx.Rollback()
		panic("transaction panic")
	}
}

package newsletters

import (
	"database/sql"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrStoreUnavailable         = errors.New("newsletter store unavailable")
	ErrNewsletterEntryNotFound  = errors.New("newsletter entry not found")
	ErrNewsletterMediaNotFound  = errors.New("newsletter media not found")
	ErrMediaBucketNotConfigured = errors.New("media bucket is not configured")
)

var (
	newsletterNowFunc    = time.Now
	uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
		return util.UploadBytesToGCS(data, bucketName, objectName, contentType)
	}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return util.ReadGCSObject(bucketName, objectName)
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error {
		return util.DeleteGCSObject(bucketName, objectName)
	}
)

type NewsletterService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

func (s *NewsletterService) ListNewsletterEntries(filter ListNewsletterFilter) (*NewsletterListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	query := s.DB.Model(&NewsletterEntry{})

	if status := strings.TrimSpace(filter.Status); status != "" {
		if !isAllowedNewsletterStatus(status) {
			return nil, fmt.Errorf("invalid status")
		}
		query = query.Where("status = ?", status)
	}

	if visibility := strings.TrimSpace(filter.Visibility); visibility != "" {
		if !isAllowedNewsletterVisibility(visibility) {
			return nil, fmt.Errorf("invalid visibility")
		}
		query = query.Where("visibility = ?", visibility)
	}

	if searchTerm := strings.TrimSpace(filter.SearchTerm); searchTerm != "" {
		pattern := "%" + strings.ToLower(searchTerm) + "%"
		query = query.Where(
			"LOWER(COALESCE(title, '')) LIKE ? OR LOWER(COALESCE(content_html, '')) LIKE ? OR LOWER(COALESCE(category, '')) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	sortBy := allowedNewsletterSortColumn(filter.SortBy)
	desc := strings.ToLower(strings.TrimSpace(filter.SortOrder)) != "asc"
	query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: sortBy}, Desc: desc})

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var entries []NewsletterEntry
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries).Error; err != nil {
		return nil, err
	}

	mediaByEntryID := make(map[int][]NewsletterMediaResponse, len(entries))
	if len(entries) > 0 {
		entryIDs := make([]int, 0, len(entries))
		for _, entry := range entries {
			entryIDs = append(entryIDs, entry.ID)
		}

		var mediaList []NewsletterMedia
		if err := s.DB.
			Where("newsletter_entry_id IN ?", entryIDs).
			Order("newsletter_entry_id ASC").
			Order("sort_order ASC").
			Order("id ASC").
			Find(&mediaList).Error; err != nil {
			return nil, err
		}

		for _, media := range mediaList {
			mediaByEntryID[media.NewsletterEntryID] = append(
				mediaByEntryID[media.NewsletterEntryID],
				newsletterMediaFromModel(media),
			)
		}
	}

	items := make([]NewsletterSummaryItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, newsletterSummaryFromModel(entry, mediaByEntryID[entry.ID]))
	}

	return &NewsletterListResponse{
		Items:      items,
		Total:      totalItems,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (totalItems + int64(pageSize) - 1) / int64(pageSize),
	}, nil
}

func (s *NewsletterService) GetNewsletterEntry(id int) (*NewsletterDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getNewsletterEntryModel(id)
	if err != nil {
		return nil, err
	}

	var mediaList []NewsletterMedia
	if err := s.DB.Where("newsletter_entry_id = ?", id).Order("sort_order ASC, id ASC").Find(&mediaList).Error; err != nil {
		return nil, err
	}

	mediaResponses := make([]NewsletterMediaResponse, 0, len(mediaList))
	for _, media := range mediaList {
		mediaResponses = append(mediaResponses, newsletterMediaFromModel(media))
	}

	resp := newsletterDetailFromModel(entry, mediaResponses)
	return &resp, nil
}

func (s *NewsletterService) GetNewsletterMediaContent(id int, mediaID int) (*NewsletterMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	if _, err := s.getNewsletterEntryModel(id); err != nil {
		return nil, err
	}

	var media NewsletterMedia
	if err := s.DB.Where("id = ? AND newsletter_entry_id = ?", mediaID, id).First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNewsletterMediaNotFound
		}
		return nil, err
	}

	objectKey := strings.TrimSpace(media.GCPObjectKey)
	bucketName, objectKey, err := s.resolveStoredObjectReference(objectKey, media.FileURL)
	if err != nil {
		return nil, err
	}

	data, contentType, err := downloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrNewsletterMediaNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = media.MimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return &NewsletterMediaContent{
		Content:     data,
		ContentType: contentType,
		FileName:    media.FileName,
	}, nil
}

func (s *NewsletterService) CreateNewsletterEntry(req SaveNewsletterEntryRequest, userID *int) (*NewsletterMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, sendDate, publishAt, err := normalizeSaveNewsletterEntryRequest(req)
	if err != nil {
		return nil, err
	}

	entry := NewsletterEntry{
		Title:       cleanReq.Title,
		Category:    cleanReq.Category,
		SendDate:    sendDate,
		ContentHTML: cleanReq.ContentHTML,
		Status:      cleanReq.Status,
		Visibility:  cleanReq.Visibility,
		PublishAt:   publishAt,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	if err := s.DB.Create(&entry).Error; err != nil {
		return nil, err
	}

	return newsletterMutationFromModel(entry), nil
}

func (s *NewsletterService) UpdateNewsletterEntry(id int, req SaveNewsletterEntryRequest, userID *int) (*NewsletterMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getNewsletterEntryModel(id)
	if err != nil {
		return nil, err
	}

	cleanReq, sendDate, publishAt, err := normalizeSaveNewsletterEntryRequest(req)
	if err != nil {
		return nil, err
	}

	entry.Title = cleanReq.Title
	entry.Category = cleanReq.Category
	entry.SendDate = sendDate
	entry.ContentHTML = cleanReq.ContentHTML
	entry.Status = cleanReq.Status
	entry.Visibility = cleanReq.Visibility
	entry.PublishAt = publishAt
	entry.UpdatedBy = userID

	if err := s.DB.Save(&entry).Error; err != nil {
		return nil, err
	}

	return newsletterMutationFromModel(entry), nil
}

func (s *NewsletterService) DeleteNewsletterEntry(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	entry, err := s.getNewsletterEntryModel(id)
	if err != nil {
		return err
	}

	var mediaList []NewsletterMedia
	if err := s.DB.Where("newsletter_entry_id = ?", id).Find(&mediaList).Error; err != nil {
		return err
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("newsletter_entry_id = ?", id).Delete(&NewsletterMedia{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entry).Error
	}); err != nil {
		return err
	}

	for _, media := range mediaList {
		s.deleteStoredObjectBestEffort(media.GCPObjectKey, media.FileURL)
	}

	return nil
}

func (s *NewsletterService) AddNewsletterMedia(id int, req AddNewsletterMediaRequest, userID *int) (*AddNewsletterMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	if len(req.Media) == 0 {
		return nil, fmt.Errorf("media is required")
	}

	if _, err := s.getNewsletterEntryModel(id); err != nil {
		return nil, err
	}

	uploadedKeys := make([]string, 0)
	uploadedCount := 0

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		nextSort, err := nextNewsletterMediaSortOrder(tx, id)
		if err != nil {
			return err
		}

		for i, input := range req.Media {
			media, uploadedKey, err := s.buildNewsletterMediaModel(id, input, userID, nextSort+i)
			if err != nil {
				return fmt.Errorf("media[%d]: %w", i, err)
			}
			if uploadedKey != "" {
				uploadedKeys = append(uploadedKeys, uploadedKey)
				uploadedCount++
			}

			if err := tx.Create(&media).Error; err != nil {
				return err
			}
		}

		return touchNewsletterEntry(tx, id, userID)
	})
	if err != nil {
		for _, key := range uploadedKeys {
			s.deleteObjectBestEffort(key)
		}
		return nil, err
	}

	return &AddNewsletterMediaResponse{UploadedCount: uploadedCount}, nil
}

func (s *NewsletterService) UpdateNewsletterMedia(id int, mediaID int, req UpdateNewsletterMediaRequest) (*NewsletterMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var media NewsletterMedia
	if err := s.DB.Where("id = ? AND newsletter_entry_id = ?", mediaID, id).First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if _, entryErr := s.getNewsletterEntryModel(id); entryErr != nil {
				return nil, entryErr
			}
			return nil, ErrNewsletterMediaNotFound
		}
		return nil, err
	}

	displayName := strings.TrimSpace(req.DisplayName)
	fileName := strings.TrimSpace(req.FileName)
	if displayName == "" && fileName == "" {
		return nil, fmt.Errorf("display_name or file_name is required")
	}

	if displayName != "" {
		media.DisplayName = displayName
	}
	if fileName != "" {
		media.FileName = fileName
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&media).Error; err != nil {
			return err
		}
		return touchNewsletterEntry(tx, id, nil)
	}); err != nil {
		return nil, err
	}

	return newsletterMediaPtrFromModel(media), nil
}

func (s *NewsletterService) ReorderNewsletterMedia(id int, mediaIDs []int) (*ReorderNewsletterMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanIDs, err := validateNewsletterMediaIDs(mediaIDs)
	if err != nil {
		return nil, err
	}
	if _, err := s.getNewsletterEntryModel(id); err != nil {
		return nil, err
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := validateNewsletterMediaReorderSet(tx, id, cleanIDs); err != nil {
			return err
		}

		for index, mediaID := range cleanIDs {
			res := tx.Model(&NewsletterMedia{}).
				Where("id = ? AND newsletter_entry_id = ?", mediaID, id).
				Update("sort_order", index)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return ErrNewsletterMediaNotFound
			}
		}

		return touchNewsletterEntry(tx, id, nil)
	})
	if err != nil {
		return nil, err
	}

	return &ReorderNewsletterMediaResponse{UpdatedCount: len(cleanIDs)}, nil
}

func (s *NewsletterService) DeleteNewsletterMedia(id int, mediaIDs []int) (*DeleteNewsletterMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanIDs, err := validateNewsletterMediaIDs(mediaIDs)
	if err != nil {
		return nil, err
	}
	if _, err := s.getNewsletterEntryModel(id); err != nil {
		return nil, err
	}

	var mediaList []NewsletterMedia
	if err := s.DB.Where("id IN ? AND newsletter_entry_id = ?", cleanIDs, id).Find(&mediaList).Error; err != nil {
		return nil, err
	}
	if len(mediaList) != len(cleanIDs) {
		return nil, ErrNewsletterMediaNotFound
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id IN ? AND newsletter_entry_id = ?", cleanIDs, id).Delete(&NewsletterMedia{}).Error; err != nil {
			return err
		}
		if err := resequenceNewsletterMedia(tx, id); err != nil {
			return err
		}
		return touchNewsletterEntry(tx, id, nil)
	}); err != nil {
		return nil, err
	}

	for _, media := range mediaList {
		s.deleteStoredObjectBestEffort(media.GCPObjectKey, media.FileURL)
	}

	return &DeleteNewsletterMediaResponse{DeletedCount: len(mediaList)}, nil
}

func normalizeSaveNewsletterEntryRequest(req SaveNewsletterEntryRequest) (SaveNewsletterEntryRequest, time.Time, *time.Time, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.ToLower(strings.TrimSpace(req.Category))
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.Visibility = strings.ToLower(strings.TrimSpace(req.Visibility))
	req.SendDate = strings.TrimSpace(req.SendDate)

	if req.Title == "" {
		return req, time.Time{}, nil, fmt.Errorf("title is required")
	}
	if len(req.Title) > 255 {
		return req, time.Time{}, nil, fmt.Errorf("title must be 255 characters or fewer")
	}
	if req.SendDate == "" {
		return req, time.Time{}, nil, fmt.Errorf("send_date is required")
	}
	if req.Category != "" && !isAllowedNewsletterCategory(req.Category) {
		return req, time.Time{}, nil, fmt.Errorf("category must be one of csaa, cst")
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if !isAllowedNewsletterStatus(req.Status) {
		return req, time.Time{}, nil, fmt.Errorf("status must be one of draft, published, scheduled")
	}
	if req.Visibility == "" {
		req.Visibility = "public"
	}
	if !isAllowedNewsletterVisibility(req.Visibility) {
		return req, time.Time{}, nil, fmt.Errorf("visibility must be one of public, private")
	}

	sendDate, err := time.Parse("2006-01-02", req.SendDate)
	if err != nil {
		return req, time.Time{}, nil, fmt.Errorf("invalid send_date format, expected YYYY-MM-DD")
	}

	publishAt, err := parseOptionalNewsletterTime(req.PublishAt)
	if err != nil {
		return req, time.Time{}, nil, err
	}
	if req.Status == "scheduled" && publishAt == nil {
		return req, time.Time{}, nil, fmt.Errorf("publish_at is required when status is scheduled")
	}

	return req, sendDate, publishAt, nil
}

func parseOptionalNewsletterTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	raw := strings.TrimSpace(*value)
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("invalid publish_at format")
}

func (s *NewsletterService) buildNewsletterMediaModel(newsletterEntryID int, input NewsletterUploadInput, userID *int, sortOrder int) (NewsletterMedia, string, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	fileName := sanitizeStoredFilename(input.FileName)
	mimeType := normalizeMimeType(input.MimeType)
	fileURL := strings.TrimSpace(input.FileURL)
	objectKey := strings.TrimSpace(input.GCPObjectKey)
	fileSize := input.FileSize
	uploadedKey := ""

	if len(input.Content) > 0 {
		if strings.TrimSpace(s.BucketName) == "" {
			return NewsletterMedia{}, "", ErrMediaBucketNotConfigured
		}

		objectKey = s.buildObjectKey("documents", fileName, userID)
		uploadedURL, uploadedSize, err := uploadBytesToGCSHook(input.Content, s.BucketName, objectKey, mimeType)
		if err != nil {
			return NewsletterMedia{}, "", err
		}
		if strings.TrimSpace(uploadedURL) == "" {
			return NewsletterMedia{}, objectKey, fmt.Errorf("media upload returned an empty file URL")
		}
		fileURL = uploadedURL
		fileSize = uploadedSize
		uploadedKey = objectKey
	}

	if fileURL == "" {
		return NewsletterMedia{}, uploadedKey, fmt.Errorf("file upload or file_url is required")
	}
	if objectKey == "" && looksLikeGCSReference(fileURL) {
		if resolvedBucket, resolvedObjectKey, parseErr := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL); parseErr == nil && strings.TrimSpace(resolvedBucket) != "" {
			objectKey = resolvedObjectKey
		}
	}
	if displayName == "" {
		displayName = fileName
	}
	if displayName == "" {
		displayName = "Newsletter document"
	}

	return NewsletterMedia{
		NewsletterEntryID: newsletterEntryID,
		DisplayName:       displayName,
		FileName:          fileName,
		GCPObjectKey:      objectKey,
		FileURL:           fileURL,
		MimeType:          mimeType,
		FileSize:          fileSize,
		MediaRole:         "attachment",
		SortOrder:         sortOrder,
		CreatedBy:         userID,
		UpdatedBy:         userID,
	}, uploadedKey, nil
}

func (s *NewsletterService) buildObjectKey(kind string, filename string, userID *int) string {
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	userPart := "anonymous"
	if userID != nil {
		userPart = strconv.Itoa(*userID)
	}

	ext := safeFileExtension(filename)
	objectName := fmt.Sprintf("%s-%d-u%s%s", kind, newsletterNowFunc().UnixNano(), userPart, ext)
	if prefix == "" {
		return path.Join("news-letters", kind, objectName)
	}
	return path.Join(prefix, "news-letters", kind, objectName)
}

func (s *NewsletterService) deleteObjectBestEffort(objectKey string) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" || strings.TrimSpace(s.BucketName) == "" {
		return
	}
	_ = deleteGCSObjectHook(s.BucketName, objectKey)
}

func (s *NewsletterService) deleteStoredObjectBestEffort(objectKey string, fileURL string) {
	bucketName, resolvedObjectKey, err := s.resolveStoredObjectReference(objectKey, fileURL)
	if err != nil || strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		return
	}
	_ = deleteGCSObjectHook(bucketName, resolvedObjectKey)
}

func (s *NewsletterService) resolveStoredObjectReference(objectKey string, fileURL string) (string, string, error) {
	objectKey = strings.TrimSpace(objectKey)
	fileURL = strings.TrimSpace(fileURL)
	if objectKey != "" && strings.TrimSpace(s.BucketName) != "" {
		return strings.TrimSpace(s.BucketName), objectKey, nil
	}
	if fileURL == "" {
		if objectKey != "" {
			return "", "", ErrMediaBucketNotConfigured
		}
		return "", "", fmt.Errorf("media content is not available from storage")
	}
	if !looksLikeGCSReference(fileURL) {
		return "", "", fmt.Errorf("media content is not available from storage")
	}

	bucketName, resolvedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL)
	if err != nil {
		if errors.Is(err, util.ErrBucketNameRequired) {
			return "", "", ErrMediaBucketNotConfigured
		}
		if errors.Is(err, util.ErrObjectNameRequired) {
			return "", "", fmt.Errorf("media content is not available from storage")
		}
		return "", "", err
	}
	if strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		return "", "", fmt.Errorf("media content is not available from storage")
	}
	return bucketName, resolvedObjectKey, nil
}

func (s *NewsletterService) getNewsletterEntryModel(id int) (NewsletterEntry, error) {
	var entry NewsletterEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewsletterEntry{}, ErrNewsletterEntryNotFound
		}
		return NewsletterEntry{}, err
	}
	return entry, nil
}

func nextNewsletterMediaSortOrder(tx *gorm.DB, newsletterEntryID int) (int, error) {
	var maxSort sql.NullInt64
	if err := tx.Model(&NewsletterMedia{}).
		Where("newsletter_entry_id = ?", newsletterEntryID).
		Select("MAX(sort_order)").
		Scan(&maxSort).Error; err != nil {
		return 0, err
	}
	if !maxSort.Valid {
		return 0, nil
	}
	return int(maxSort.Int64) + 1, nil
}

func validateNewsletterMediaReorderSet(tx *gorm.DB, newsletterEntryID int, mediaIDs []int) error {
	var rows []NewsletterMedia
	if err := tx.
		Where("newsletter_entry_id = ?", newsletterEntryID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrNewsletterMediaNotFound
	}

	available := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		available[row.ID] = struct{}{}
	}
	for _, mediaID := range mediaIDs {
		if _, exists := available[mediaID]; !exists {
			return ErrNewsletterMediaNotFound
		}
	}
	if len(rows) != len(mediaIDs) {
		return fmt.Errorf("media_ids must include every newsletter media item exactly once")
	}
	return nil
}

func touchNewsletterEntry(tx *gorm.DB, newsletterEntryID int, userID *int) error {
	updates := map[string]interface{}{
		"updated_at": newsletterNowFunc(),
	}
	if userID != nil {
		updates["updated_by"] = userID
	}
	return tx.Model(&NewsletterEntry{}).Where("id = ?", newsletterEntryID).Updates(updates).Error
}

func resequenceNewsletterMedia(tx *gorm.DB, newsletterEntryID int) error {
	var mediaList []NewsletterMedia
	if err := tx.
		Where("newsletter_entry_id = ?", newsletterEntryID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&mediaList).Error; err != nil {
		return err
	}

	for index, media := range mediaList {
		if media.SortOrder == index {
			continue
		}
		if err := tx.Model(&NewsletterMedia{}).
			Where("id = ? AND newsletter_entry_id = ?", media.ID, newsletterEntryID).
			Update("sort_order", index).Error; err != nil {
			return err
		}
	}

	return nil
}

func validateNewsletterMediaIDs(mediaIDs []int) ([]int, error) {
	if len(mediaIDs) == 0 {
		return nil, fmt.Errorf("media_ids is required")
	}

	seen := make(map[int]struct{}, len(mediaIDs))
	cleanIDs := make([]int, 0, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		if mediaID <= 0 {
			return nil, fmt.Errorf("media_ids must contain positive integers")
		}
		if _, exists := seen[mediaID]; exists {
			return nil, fmt.Errorf("media_ids must not contain duplicates")
		}
		seen[mediaID] = struct{}{}
		cleanIDs = append(cleanIDs, mediaID)
	}

	return cleanIDs, nil
}

func isAllowedNewsletterStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "draft", "published", "scheduled":
		return true
	default:
		return false
	}
}

func isAllowedNewsletterVisibility(visibility string) bool {
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case "public", "private":
		return true
	default:
		return false
	}
}

func isAllowedNewsletterCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "csaa", "cst":
		return true
	default:
		return false
	}
}

func allowedNewsletterSortColumn(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "title":
		return "title"
	case "created_at":
		return "created_at"
	case "updated_at":
		return "updated_at"
	case "status":
		return "status"
	case "visibility":
		return "visibility"
	case "category":
		return "category"
	default:
		return "send_date"
	}
}

func newsletterSummaryFromModel(entry NewsletterEntry, media []NewsletterMediaResponse) NewsletterSummaryItem {
	if media == nil {
		media = []NewsletterMediaResponse{}
	}

	return NewsletterSummaryItem{
		ID:          entry.ID,
		Title:       entry.Title,
		Category:    entry.Category,
		SendDate:    entry.SendDate,
		ContentHTML: entry.ContentHTML,
		Status:      entry.Status,
		Visibility:  entry.Visibility,
		PublishAt:   entry.PublishAt,
		Media:       media,
		CreatedBy:   entry.CreatedBy,
		UpdatedBy:   entry.UpdatedBy,
		CreatedAt:   entry.CreatedAt,
		UpdatedAt:   entry.UpdatedAt,
	}
}

func newsletterDetailFromModel(entry NewsletterEntry, media []NewsletterMediaResponse) NewsletterDetailResponse {
	if media == nil {
		media = []NewsletterMediaResponse{}
	}

	return NewsletterDetailResponse{
		ID:          entry.ID,
		Title:       entry.Title,
		Category:    entry.Category,
		SendDate:    entry.SendDate,
		ContentHTML: entry.ContentHTML,
		Status:      entry.Status,
		Visibility:  entry.Visibility,
		PublishAt:   entry.PublishAt,
		Media:       media,
		CreatedBy:   entry.CreatedBy,
		UpdatedBy:   entry.UpdatedBy,
		CreatedAt:   entry.CreatedAt,
		UpdatedAt:   entry.UpdatedAt,
	}
}

func newsletterMutationFromModel(entry NewsletterEntry) *NewsletterMutationResponse {
	return &NewsletterMutationResponse{
		ID:         entry.ID,
		Title:      entry.Title,
		Category:   entry.Category,
		SendDate:   entry.SendDate,
		Status:     entry.Status,
		Visibility: entry.Visibility,
	}
}

func newsletterMediaPtrFromModel(media NewsletterMedia) *NewsletterMediaResponse {
	response := newsletterMediaFromModel(media)
	return &response
}

func newsletterMediaFromModel(media NewsletterMedia) NewsletterMediaResponse {
	return NewsletterMediaResponse{
		ID:           media.ID,
		DisplayName:  media.DisplayName,
		FileName:     media.FileName,
		GCPObjectKey: media.GCPObjectKey,
		FileURL:      media.FileURL,
		MimeType:     media.MimeType,
		FileSize:     media.FileSize,
		MediaRole:    media.MediaRole,
		SortOrder:    media.SortOrder,
		CreatedBy:    media.CreatedBy,
		UpdatedBy:    media.UpdatedBy,
		CreatedAt:    media.CreatedAt,
		UpdatedAt:    media.UpdatedAt,
	}
}

func normalizeMimeType(mimeType string) string {
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

func sanitizeStoredFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = path.Base(filename)
	if filename == "." || filename == "/" {
		return ""
	}
	return filename
}

func safeFileExtension(filename string) string {
	filename = sanitizeStoredFilename(filename)
	ext := strings.ToLower(filepath.Ext(filename))
	if len(ext) > 20 {
		return ""
	}
	return ext
}

func looksLikeGCSReference(fileURL string) bool {
	fileURL = strings.ToLower(strings.TrimSpace(fileURL))
	switch {
	case fileURL == "":
		return false
	case strings.HasPrefix(fileURL, "gs://"):
		return true
	case strings.HasPrefix(fileURL, "https://storage.googleapis.com/"),
		strings.HasPrefix(fileURL, "http://storage.googleapis.com/"),
		strings.Contains(fileURL, ".storage.googleapis.com/"):
		return true
	default:
		return !strings.Contains(fileURL, "://")
	}
}

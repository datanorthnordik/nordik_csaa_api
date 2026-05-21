package press

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
	ErrStoreUnavailable         = errors.New("press store unavailable")
	ErrPressEntryNotFound       = errors.New("press entry not found")
	ErrPressMediaNotFound       = errors.New("press media not found")
	ErrMediaBucketNotConfigured = errors.New("media bucket is not configured")
)

var (
	pressNowFunc         = time.Now
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

type PressService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

func (s *PressService) ListPressEntries(filter ListPressFilter) (*PressListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	query := s.DB.Model(&PressEntry{})

	if status := strings.TrimSpace(filter.Status); status != "" {
		if !isAllowedPressStatus(status) {
			return nil, fmt.Errorf("invalid status")
		}
		query = query.Where("status = ?", status)
	}

	if visibility := strings.TrimSpace(filter.Visibility); visibility != "" {
		if !isAllowedPressVisibility(visibility) {
			return nil, fmt.Errorf("invalid visibility")
		}
		query = query.Where("visibility = ?", visibility)
	}

	if searchTerm := strings.TrimSpace(filter.SearchTerm); searchTerm != "" {
		pattern := "%" + strings.ToLower(searchTerm) + "%"
		query = query.Where(
			"LOWER(COALESCE(title, '')) LIKE ? OR LOWER(COALESCE(content_html, '')) LIKE ? OR LOWER(COALESCE(source_url, '')) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	sortBy := allowedPressSortColumn(filter.SortBy)
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

	var entries []PressEntry
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries).Error; err != nil {
		return nil, err
	}

	mediaByEntryID := make(map[int][]PressMediaResponse, len(entries))
	if len(entries) > 0 {
		entryIDs := make([]int, 0, len(entries))
		for _, entry := range entries {
			entryIDs = append(entryIDs, entry.ID)
		}

		var mediaList []PressMedia
		if err := s.DB.
			Where("press_entry_id IN ?", entryIDs).
			Order("press_entry_id ASC").
			Order("sort_order ASC").
			Order("id ASC").
			Find(&mediaList).Error; err != nil {
			return nil, err
		}

		for _, media := range mediaList {
			mediaByEntryID[media.PressEntryID] = append(
				mediaByEntryID[media.PressEntryID],
				pressMediaFromModel(media),
			)
		}
	}

	items := make([]PressSummaryItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, pressSummaryFromModel(entry, mediaByEntryID[entry.ID]))
	}

	return &PressListResponse{
		Items:      items,
		Total:      totalItems,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (totalItems + int64(pageSize) - 1) / int64(pageSize),
	}, nil
}

func (s *PressService) GetPressEntry(id int) (*PressDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getPressEntryModel(id)
	if err != nil {
		return nil, err
	}

	var mediaList []PressMedia
	if err := s.DB.Where("press_entry_id = ?", id).Order("sort_order ASC, id ASC").Find(&mediaList).Error; err != nil {
		return nil, err
	}

	mediaResponses := make([]PressMediaResponse, 0, len(mediaList))
	for _, media := range mediaList {
		mediaResponses = append(mediaResponses, pressMediaFromModel(media))
	}

	resp := pressDetailFromModel(entry, mediaResponses)
	return &resp, nil
}

func (s *PressService) GetPressCoverImageContent(id int) (*PressMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getPressEntryModel(id)
	if err != nil {
		return nil, err
	}

	objectKey := strings.TrimSpace(entry.CoverImageGCPKey)
	fileURL := strings.TrimSpace(entry.CoverImageURL)
	if objectKey == "" && fileURL == "" {
		return nil, ErrPressMediaNotFound
	}

	bucketName, resolvedObjectKey, err := s.resolveStoredObjectReference(objectKey, fileURL)
	if err != nil {
		return nil, err
	}

	data, contentType, err := downloadGCSObjectHook(bucketName, resolvedObjectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrPressMediaNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return &PressMediaContent{
		Content:     data,
		ContentType: contentType,
		FileName:    storedFilename(resolvedObjectKey, "cover-image"),
	}, nil
}

func (s *PressService) GetPressMediaContent(id int, mediaID int) (*PressMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	if _, err := s.getPressEntryModel(id); err != nil {
		return nil, err
	}

	var media PressMedia
	if err := s.DB.Where("id = ? AND press_entry_id = ?", mediaID, id).First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressMediaNotFound
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
			return nil, ErrPressMediaNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = media.MimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return &PressMediaContent{
		Content:     data,
		ContentType: contentType,
		FileName:    media.FileName,
	}, nil
}

func (s *PressService) CreatePressEntry(req SavePressEntryRequest, userID *int) (*PressMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, releaseDate, publishAt, err := normalizeSavePressEntryRequest(req)
	if err != nil {
		return nil, err
	}

	entry := PressEntry{
		Title:       cleanReq.Title,
		ReleaseDate: releaseDate,
		CategoryID:  cleanReq.CategoryID,
		SourceURL:   cleanReq.SourceURL,
		ContentHTML: cleanReq.ContentHTML,
		Status:      cleanReq.Status,
		Visibility:  cleanReq.Visibility,
		PublishAt:   publishAt,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	uploadedCoverKey := ""
	if cleanReq.CoverImage != nil {
		coverURL, coverKey, wasUploaded, err := s.resolveCoverImage(*cleanReq.CoverImage, userID)
		if err != nil {
			return nil, err
		}
		entry.CoverImageURL = coverURL
		entry.CoverImageGCPKey = coverKey
		if wasUploaded {
			uploadedCoverKey = coverKey
		}
	}

	if err := s.DB.Create(&entry).Error; err != nil {
		s.deleteObjectBestEffort(uploadedCoverKey)
		return nil, err
	}

	return pressMutationFromModel(entry), nil
}

func (s *PressService) UpdatePressEntry(id int, req SavePressEntryRequest, userID *int) (*PressMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getPressEntryModel(id)
	if err != nil {
		return nil, err
	}

	cleanReq, releaseDate, publishAt, err := normalizeSavePressEntryRequest(req)
	if err != nil {
		return nil, err
	}

	oldCoverKey := entry.CoverImageGCPKey
	oldCoverURL := entry.CoverImageURL
	newCoverKey := ""
	replaceOrRemoveCover := cleanReq.RemoveCoverImage

	if !cleanReq.RemoveCoverImage && cleanReq.CoverImage != nil {
		coverURL, coverKey, wasUploaded, err := s.resolveCoverImage(*cleanReq.CoverImage, userID)
		if err != nil {
			return nil, err
		}
		entry.CoverImageURL = coverURL
		entry.CoverImageGCPKey = coverKey
		replaceOrRemoveCover = true
		if wasUploaded {
			newCoverKey = coverKey
		}
	}

	if cleanReq.RemoveCoverImage {
		entry.CoverImageURL = ""
		entry.CoverImageGCPKey = ""
	}

	entry.Title = cleanReq.Title
	entry.ReleaseDate = releaseDate
	entry.CategoryID = cleanReq.CategoryID
	entry.SourceURL = cleanReq.SourceURL
	entry.ContentHTML = cleanReq.ContentHTML
	entry.Status = cleanReq.Status
	entry.Visibility = cleanReq.Visibility
	entry.PublishAt = publishAt
	entry.UpdatedBy = userID

	if err := s.DB.Save(&entry).Error; err != nil {
		s.deleteObjectBestEffort(newCoverKey)
		return nil, err
	}

	if replaceOrRemoveCover && (oldCoverKey != "" || strings.TrimSpace(oldCoverURL) != "") && oldCoverKey != newCoverKey {
		s.deleteStoredObjectBestEffort(oldCoverKey, oldCoverURL)
	}

	return pressMutationFromModel(entry), nil
}

func (s *PressService) DeletePressEntry(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	entry, err := s.getPressEntryModel(id)
	if err != nil {
		return err
	}

	var mediaList []PressMedia
	if err := s.DB.Where("press_entry_id = ?", id).Find(&mediaList).Error; err != nil {
		return err
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("press_entry_id = ?", id).Delete(&PressMedia{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entry).Error
	}); err != nil {
		return err
	}

	s.deleteStoredObjectBestEffort(entry.CoverImageGCPKey, entry.CoverImageURL)
	for _, media := range mediaList {
		s.deleteStoredObjectBestEffort(media.GCPObjectKey, media.FileURL)
	}

	return nil
}

func (s *PressService) AddPressMedia(id int, req AddPressMediaRequest, userID *int) (*AddPressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	if len(req.Media) == 0 {
		return nil, fmt.Errorf("media is required")
	}

	if _, err := s.getPressEntryModel(id); err != nil {
		return nil, err
	}

	uploadedKeys := make([]string, 0)
	uploadedCount := 0

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		nextSort, err := nextPressMediaSortOrder(tx, id)
		if err != nil {
			return err
		}

		for i, input := range req.Media {
			media, uploadedKey, err := s.buildPressMediaModel(id, input, userID, nextSort+i)
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

		return touchPressEntry(tx, id, userID)
	})
	if err != nil {
		for _, key := range uploadedKeys {
			s.deleteObjectBestEffort(key)
		}
		return nil, err
	}

	return &AddPressMediaResponse{UploadedCount: uploadedCount}, nil
}

func (s *PressService) UpdatePressMedia(id int, mediaID int, req UpdatePressMediaRequest) (*PressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var media PressMedia
	if err := s.DB.Where("id = ? AND press_entry_id = ?", mediaID, id).First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if _, entryErr := s.getPressEntryModel(id); entryErr != nil {
				return nil, entryErr
			}
			return nil, ErrPressMediaNotFound
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
		return touchPressEntry(tx, id, nil)
	}); err != nil {
		return nil, err
	}

	return pressMediaPtrFromModel(media), nil
}

func (s *PressService) ReorderPressMedia(id int, mediaIDs []int) (*ReorderPressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanIDs, err := validatePressMediaIDs(mediaIDs)
	if err != nil {
		return nil, err
	}
	if _, err := s.getPressEntryModel(id); err != nil {
		return nil, err
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := validatePressMediaReorderSet(tx, id, cleanIDs); err != nil {
			return err
		}

		for index, mediaID := range cleanIDs {
			res := tx.Model(&PressMedia{}).
				Where("id = ? AND press_entry_id = ?", mediaID, id).
				Update("sort_order", index)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return ErrPressMediaNotFound
			}
		}

		return touchPressEntry(tx, id, nil)
	})
	if err != nil {
		return nil, err
	}

	return &ReorderPressMediaResponse{UpdatedCount: len(cleanIDs)}, nil
}

func (s *PressService) DeletePressMedia(id int, mediaIDs []int) (*DeletePressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanIDs, err := validatePressMediaIDs(mediaIDs)
	if err != nil {
		return nil, err
	}
	if _, err := s.getPressEntryModel(id); err != nil {
		return nil, err
	}

	var mediaList []PressMedia
	if err := s.DB.Where("id IN ? AND press_entry_id = ?", cleanIDs, id).Find(&mediaList).Error; err != nil {
		return nil, err
	}
	if len(mediaList) != len(cleanIDs) {
		return nil, ErrPressMediaNotFound
	}

	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id IN ? AND press_entry_id = ?", cleanIDs, id).Delete(&PressMedia{}).Error; err != nil {
			return err
		}
		if err := resequencePressMedia(tx, id); err != nil {
			return err
		}
		return touchPressEntry(tx, id, nil)
	}); err != nil {
		return nil, err
	}

	for _, media := range mediaList {
		s.deleteStoredObjectBestEffort(media.GCPObjectKey, media.FileURL)
	}

	return &DeletePressMediaResponse{DeletedCount: len(mediaList)}, nil
}

func normalizeSavePressEntryRequest(req SavePressEntryRequest) (SavePressEntryRequest, time.Time, *time.Time, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.SourceURL = strings.TrimSpace(req.SourceURL)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.Visibility = strings.ToLower(strings.TrimSpace(req.Visibility))
	req.ReleaseDate = strings.TrimSpace(req.ReleaseDate)

	if req.Title == "" {
		return req, time.Time{}, nil, fmt.Errorf("title is required")
	}
	if len(req.Title) > 255 {
		return req, time.Time{}, nil, fmt.Errorf("title must be 255 characters or fewer")
	}
	if req.ReleaseDate == "" {
		return req, time.Time{}, nil, fmt.Errorf("release_date is required")
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if !isAllowedPressStatus(req.Status) {
		return req, time.Time{}, nil, fmt.Errorf("status must be one of draft, published, archived")
	}
	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if !isAllowedPressVisibility(req.Visibility) {
		return req, time.Time{}, nil, fmt.Errorf("visibility must be one of public, private, scheduled")
	}
	if req.CategoryID != nil && *req.CategoryID <= 0 {
		return req, time.Time{}, nil, fmt.Errorf("category_id must be positive")
	}

	releaseDate, err := time.Parse("2006-01-02", req.ReleaseDate)
	if err != nil {
		return req, time.Time{}, nil, fmt.Errorf("invalid release_date format, expected YYYY-MM-DD")
	}

	publishAt, err := parseOptionalPressTime(req.PublishAt)
	if err != nil {
		return req, time.Time{}, nil, err
	}
	if req.Visibility == "scheduled" && publishAt == nil {
		return req, time.Time{}, nil, fmt.Errorf("publish_at is required when visibility is scheduled")
	}

	return req, releaseDate, publishAt, nil
}

func parseOptionalPressTime(value *string) (*time.Time, error) {
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

func (s *PressService) resolveCoverImage(input PressUploadInput, userID *int) (fileURL string, objectKey string, wasUploaded bool, err error) {
	if len(input.Content) > 0 {
		if strings.TrimSpace(s.BucketName) == "" {
			return "", "", false, ErrMediaBucketNotConfigured
		}

		objectKey = s.buildObjectKey("covers", input.FileName, userID)
		fileURL, _, err = uploadBytesToGCSHook(input.Content, s.BucketName, objectKey, normalizeMimeType(input.MimeType))
		if err != nil {
			return "", "", false, err
		}
		if strings.TrimSpace(fileURL) == "" {
			return "", "", false, fmt.Errorf("cover image upload returned an empty file URL")
		}
		return strings.TrimSpace(fileURL), objectKey, true, nil
	}

	fileURL = strings.TrimSpace(input.FileURL)
	objectKey = strings.TrimSpace(input.GCPObjectKey)
	if objectKey == "" && looksLikeGCSReference(fileURL) {
		if resolvedBucket, resolvedObjectKey, parseErr := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL); parseErr == nil && strings.TrimSpace(resolvedBucket) != "" {
			objectKey = resolvedObjectKey
		}
	}
	if fileURL == "" && objectKey == "" {
		return "", "", false, nil
	}
	return fileURL, objectKey, false, nil
}

func (s *PressService) buildPressMediaModel(pressEntryID int, input PressUploadInput, userID *int, sortOrder int) (PressMedia, string, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	fileName := sanitizeStoredFilename(input.FileName)
	mimeType := normalizeMimeType(input.MimeType)
	fileURL := strings.TrimSpace(input.FileURL)
	objectKey := strings.TrimSpace(input.GCPObjectKey)
	fileSize := input.FileSize
	uploadedKey := ""

	if len(input.Content) > 0 {
		if strings.TrimSpace(s.BucketName) == "" {
			return PressMedia{}, "", ErrMediaBucketNotConfigured
		}

		objectKey = s.buildObjectKey("media", fileName, userID)
		uploadedURL, uploadedSize, err := uploadBytesToGCSHook(input.Content, s.BucketName, objectKey, mimeType)
		if err != nil {
			return PressMedia{}, "", err
		}
		if strings.TrimSpace(uploadedURL) == "" {
			return PressMedia{}, objectKey, fmt.Errorf("media upload returned an empty file URL")
		}
		fileURL = uploadedURL
		fileSize = uploadedSize
		uploadedKey = objectKey
	}

	if fileURL == "" {
		return PressMedia{}, uploadedKey, fmt.Errorf("file upload or file_url is required")
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
		displayName = "Press media"
	}

	return PressMedia{
		PressEntryID: pressEntryID,
		DisplayName:  displayName,
		FileName:     fileName,
		GCPObjectKey: objectKey,
		FileURL:      fileURL,
		MimeType:     mimeType,
		FileSize:     fileSize,
		MediaRole:    "attachment",
		SortOrder:    sortOrder,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}, uploadedKey, nil
}

func (s *PressService) buildObjectKey(kind string, filename string, userID *int) string {
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	userPart := "anonymous"
	if userID != nil {
		userPart = strconv.Itoa(*userID)
	}

	ext := safeFileExtension(filename)
	objectName := fmt.Sprintf("%s-%d-u%s%s", kind, pressNowFunc().UnixNano(), userPart, ext)
	if prefix == "" {
		return path.Join("press-entries", kind, objectName)
	}
	return path.Join(prefix, "press-entries", kind, objectName)
}

func (s *PressService) deleteObjectBestEffort(objectKey string) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" || strings.TrimSpace(s.BucketName) == "" {
		return
	}
	_ = deleteGCSObjectHook(s.BucketName, objectKey)
}

func (s *PressService) deleteStoredObjectBestEffort(objectKey string, fileURL string) {
	bucketName, resolvedObjectKey, err := s.resolveStoredObjectReference(objectKey, fileURL)
	if err != nil || strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		return
	}
	_ = deleteGCSObjectHook(bucketName, resolvedObjectKey)
}

func (s *PressService) resolveStoredObjectReference(objectKey string, fileURL string) (string, string, error) {
	objectKey = strings.TrimSpace(objectKey)
	fileURL = strings.TrimSpace(fileURL)
	if objectKey != "" && strings.TrimSpace(s.BucketName) != "" {
		bucketName := strings.TrimSpace(s.BucketName)
		return bucketName, objectKey, nil
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

func (s *PressService) getPressEntryModel(id int) (PressEntry, error) {
	var entry PressEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PressEntry{}, ErrPressEntryNotFound
		}
		return PressEntry{}, err
	}
	return entry, nil
}

func nextPressMediaSortOrder(tx *gorm.DB, pressEntryID int) (int, error) {
	var maxSort sql.NullInt64
	if err := tx.Model(&PressMedia{}).
		Where("press_entry_id = ?", pressEntryID).
		Select("MAX(sort_order)").
		Scan(&maxSort).Error; err != nil {
		return 0, err
	}
	if !maxSort.Valid {
		return 0, nil
	}
	return int(maxSort.Int64) + 1, nil
}

func validatePressMediaReorderSet(tx *gorm.DB, pressEntryID int, mediaIDs []int) error {
	var rows []PressMedia
	if err := tx.
		Where("press_entry_id = ?", pressEntryID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrPressMediaNotFound
	}

	available := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		available[row.ID] = struct{}{}
	}
	for _, mediaID := range mediaIDs {
		if _, exists := available[mediaID]; !exists {
			return ErrPressMediaNotFound
		}
	}
	if len(rows) != len(mediaIDs) {
		return fmt.Errorf("media_ids must include every press media item exactly once")
	}
	return nil
}

func touchPressEntry(tx *gorm.DB, pressEntryID int, userID *int) error {
	updates := map[string]interface{}{
		"updated_at": pressNowFunc(),
	}
	if userID != nil {
		updates["updated_by"] = userID
	}
	return tx.Model(&PressEntry{}).Where("id = ?", pressEntryID).Updates(updates).Error
}

func resequencePressMedia(tx *gorm.DB, pressEntryID int) error {
	var mediaList []PressMedia
	if err := tx.
		Where("press_entry_id = ?", pressEntryID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&mediaList).Error; err != nil {
		return err
	}

	for index, media := range mediaList {
		if media.SortOrder == index {
			continue
		}
		if err := tx.Model(&PressMedia{}).
			Where("id = ? AND press_entry_id = ?", media.ID, pressEntryID).
			Update("sort_order", index).Error; err != nil {
			return err
		}
	}

	return nil
}

func validatePressMediaIDs(mediaIDs []int) ([]int, error) {
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

func isAllowedPressStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "draft", "published", "archived":
		return true
	default:
		return false
	}
}

func isAllowedPressVisibility(visibility string) bool {
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case "public", "private", "scheduled":
		return true
	default:
		return false
	}
}

func allowedPressSortColumn(sortBy string) string {
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
	default:
		return "release_date"
	}
}

func pressSummaryFromModel(entry PressEntry, media []PressMediaResponse) PressSummaryItem {
	if media == nil {
		media = []PressMediaResponse{}
	}

	return PressSummaryItem{
		ID:               entry.ID,
		Title:            entry.Title,
		ReleaseDate:      entry.ReleaseDate,
		CategoryID:       entry.CategoryID,
		SourceURL:        entry.SourceURL,
		ContentHTML:      entry.ContentHTML,
		Status:           entry.Status,
		Visibility:       entry.Visibility,
		CoverImageURL:    entry.CoverImageURL,
		CoverImageGCPKey: entry.CoverImageGCPKey,
		PublishAt:        entry.PublishAt,
		Media:            media,
		CreatedBy:        entry.CreatedBy,
		UpdatedBy:        entry.UpdatedBy,
		CreatedAt:        entry.CreatedAt,
		UpdatedAt:        entry.UpdatedAt,
	}
}

func pressDetailFromModel(entry PressEntry, media []PressMediaResponse) PressDetailResponse {
	if media == nil {
		media = []PressMediaResponse{}
	}

	return PressDetailResponse{
		ID:               entry.ID,
		Title:            entry.Title,
		ReleaseDate:      entry.ReleaseDate,
		CategoryID:       entry.CategoryID,
		SourceURL:        entry.SourceURL,
		ContentHTML:      entry.ContentHTML,
		Status:           entry.Status,
		Visibility:       entry.Visibility,
		CoverImageURL:    entry.CoverImageURL,
		CoverImageGCPKey: entry.CoverImageGCPKey,
		PublishAt:        entry.PublishAt,
		Media:            media,
		CreatedBy:        entry.CreatedBy,
		UpdatedBy:        entry.UpdatedBy,
		CreatedAt:        entry.CreatedAt,
		UpdatedAt:        entry.UpdatedAt,
	}
}

func pressMutationFromModel(entry PressEntry) *PressMutationResponse {
	return &PressMutationResponse{
		ID:          entry.ID,
		Title:       entry.Title,
		ReleaseDate: entry.ReleaseDate,
		Status:      entry.Status,
		Visibility:  entry.Visibility,
	}
}

func pressMediaPtrFromModel(media PressMedia) *PressMediaResponse {
	response := pressMediaFromModel(media)
	return &response
}

func pressMediaFromModel(media PressMedia) PressMediaResponse {
	return PressMediaResponse{
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

func storedFilename(objectKey string, fallback string) string {
	name := path.Base(strings.TrimSpace(objectKey))
	if name == "" || name == "." || name == "/" {
		return fallback
	}
	return name
}

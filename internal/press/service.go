package press

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable         = errors.New("press store unavailable")
	ErrPressEntryNotFound       = errors.New("press entry not found")
	ErrPressMediaNotFound       = errors.New("press media not found")
	ErrMediaBucketNotConfigured = errors.New("media bucket is not configured")
)

var (
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

// ListPressEntries retrieves a filtered list of press entries
func (s *PressService) ListPressEntries(filter ListPressFilter) (*PressListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	query := s.DB.Model(&PressEntry{})

	// Apply filters
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if strings.TrimSpace(filter.Visibility) != "" {
		query = query.Where("visibility = ?", filter.Visibility)
	}

	if strings.TrimSpace(filter.SearchTerm) != "" {
		searchTerm := "%" + filter.SearchTerm + "%"
		query = query.Where("title ILIKE ? OR content_html ILIKE ? OR source_url ILIKE ?",
			searchTerm, searchTerm, searchTerm)
	}

	// Count total items
	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	// Apply sorting
	sortBy := strings.TrimSpace(filter.SortBy)
	if sortBy == "" {
		sortBy = "release_date"
	}

	sortOrder := strings.TrimSpace(filter.SortOrder)
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	offset := (filter.Page - 1) * filter.PageSize
	var entries []PressEntry
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&entries).Error; err != nil {
		return nil, err
	}

	// Convert to response format
	items := make([]PressSummaryItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, PressSummaryItem{
			ID:            entry.ID,
			Title:         entry.Title,
			ReleaseDate:   entry.ReleaseDate,
			Status:        entry.Status,
			Visibility:    entry.Visibility,
			CoverImageURL: entry.CoverImageURL,
			CreatedAt:     entry.CreatedAt,
			UpdatedAt:     entry.UpdatedAt,
		})
	}

	totalPages := (totalItems + int64(filter.PageSize) - 1) / int64(filter.PageSize)

	return &PressListResponse{
		Items:      items,
		Total:      totalItems,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetPressEntry retrieves a single press entry by ID with all media
func (s *PressService) GetPressEntry(id int) (*PressDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var entry PressEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressEntryNotFound
		}
		return nil, err
	}

	// Fetch associated media
	var mediaList []PressMedia
	if err := s.DB.Where("press_entry_id = ?", id).Order("sort_order ASC, id ASC").Find(&mediaList).Error; err != nil {
		return nil, err
	}

	mediaResponses := make([]PressMediaResponse, 0, len(mediaList))
	for _, media := range mediaList {
		mediaResponses = append(mediaResponses, PressMediaResponse{
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
		})
	}

	return &PressDetailResponse{
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
		Media:            mediaResponses,
		CreatedBy:        entry.CreatedBy,
		UpdatedBy:        entry.UpdatedBy,
		CreatedAt:        entry.CreatedAt,
		UpdatedAt:        entry.UpdatedAt,
	}, nil
}

// GetPressMediaContent retrieves the actual file content from storage
func (s *PressService) GetPressMediaContent(id int, mediaID int) (*PressMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	// Verify press entry exists
	if err := s.DB.Model(&PressEntry{}).Where("id = ?", id).First(&PressEntry{}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressEntryNotFound
		}
		return nil, err
	}

	// Get media
	var media PressMedia
	if err := s.DB.Where("id = ? AND press_entry_id = ?", mediaID, id).First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressMediaNotFound
		}
		return nil, err
	}

	// Download from GCS if object key exists
	if strings.TrimSpace(media.GCPObjectKey) != "" {
		data, contentType, err := downloadGCSObjectHook(s.BucketName, media.GCPObjectKey)
		if err != nil {
			return nil, err
		}
		return &PressMediaContent{
			Content:     data,
			ContentType: contentType,
			FileName:    media.FileName,
		}, nil
	}

	return &PressMediaContent{
		Content:     []byte{},
		ContentType: media.MimeType,
		FileName:    media.FileName,
	}, nil
}

// CreatePressEntry creates a new press entry
func (s *PressService) CreatePressEntry(req SavePressEntryRequest, userID *int) (*PressMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	// Parse release date
	releaseDate, err := time.Parse("2006-01-02", req.ReleaseDate)
	if err != nil {
		return nil, fmt.Errorf("invalid release_date format, expected YYYY-MM-DD")
	}

	var publishAt *time.Time
	if req.PublishAt != nil && strings.TrimSpace(*req.PublishAt) != "" {
		t, err := time.Parse("2006-01-02T15:04:05Z07:00", *req.PublishAt)
		if err != nil {
			return nil, fmt.Errorf("invalid publish_at format")
		}
		publishAt = &t
	}

	entry := PressEntry{
		Title:       req.Title,
		ReleaseDate: releaseDate,
		CategoryID:  req.CategoryID,
		SourceURL:   req.SourceURL,
		ContentHTML: req.ContentHTML,
		Status:      req.Status,
		Visibility:  req.Visibility,
		PublishAt:   publishAt,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	// Handle cover image upload
	if req.CoverImage != nil && len(req.CoverImage.Content) > 0 {
		if s.BucketName == "" {
			return nil, ErrMediaBucketNotConfigured
		}

		objectKey := path.Join(s.BucketPrefix, "press-entries", fmt.Sprintf("cover-%d-%d", time.Now().UnixNano(), userID))
		fileURL, _, err := uploadBytesToGCSHook(req.CoverImage.Content, s.BucketName, objectKey, req.CoverImage.MimeType)
		if err != nil {
			return nil, err
		}

		entry.CoverImageURL = fileURL
		entry.CoverImageGCPKey = objectKey
	}

	// Create entry in database
	if err := s.DB.Create(&entry).Error; err != nil {
		return nil, err
	}

	return &PressMutationResponse{
		ID:          entry.ID,
		Title:       entry.Title,
		ReleaseDate: entry.ReleaseDate,
		Status:      entry.Status,
		Visibility:  entry.Visibility,
	}, nil
}

// UpdatePressEntry updates an existing press entry
func (s *PressService) UpdatePressEntry(id int, req SavePressEntryRequest, userID *int) (*PressMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var entry PressEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressEntryNotFound
		}
		return nil, err
	}

	// Parse release date
	releaseDate, err := time.Parse("2006-01-02", req.ReleaseDate)
	if err != nil {
		return nil, fmt.Errorf("invalid release_date format, expected YYYY-MM-DD")
	}

	var publishAt *time.Time
	if req.PublishAt != nil && strings.TrimSpace(*req.PublishAt) != "" {
		t, err := time.Parse("2006-01-02T15:04:05Z07:00", *req.PublishAt)
		if err != nil {
			return nil, fmt.Errorf("invalid publish_at format")
		}
		publishAt = &t
	}

	// Update fields
	entry.Title = req.Title
	entry.ReleaseDate = releaseDate
	entry.CategoryID = req.CategoryID
	entry.SourceURL = req.SourceURL
	entry.ContentHTML = req.ContentHTML
	entry.Status = req.Status
	entry.Visibility = req.Visibility
	entry.PublishAt = publishAt
	entry.UpdatedBy = userID

	// Handle cover image
	if req.RemoveCoverImage {
		if entry.CoverImageGCPKey != "" {
			_ = deleteGCSObjectHook(s.BucketName, entry.CoverImageGCPKey)
		}
		entry.CoverImageURL = ""
		entry.CoverImageGCPKey = ""
	} else if req.CoverImage != nil && len(req.CoverImage.Content) > 0 {
		if s.BucketName == "" {
			return nil, ErrMediaBucketNotConfigured
		}

		// Delete old cover if exists
		if entry.CoverImageGCPKey != "" {
			_ = deleteGCSObjectHook(s.BucketName, entry.CoverImageGCPKey)
		}

		objectKey := path.Join(s.BucketPrefix, "press-entries", fmt.Sprintf("cover-%d-%d", time.Now().UnixNano(), userID))
		fileURL, _, err := uploadBytesToGCSHook(req.CoverImage.Content, s.BucketName, objectKey, req.CoverImage.MimeType)
		if err != nil {
			return nil, err
		}

		entry.CoverImageURL = fileURL
		entry.CoverImageGCPKey = objectKey
	}

	// Save changes
	if err := s.DB.Save(&entry).Error; err != nil {
		return nil, err
	}

	return &PressMutationResponse{
		ID:          entry.ID,
		Title:       entry.Title,
		ReleaseDate: entry.ReleaseDate,
		Status:      entry.Status,
		Visibility:  entry.Visibility,
	}, nil
}

// DeletePressEntry deletes a press entry and all its media
func (s *PressService) DeletePressEntry(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	var entry PressEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPressEntryNotFound
		}
		return err
	}

	// Delete cover image from GCS
	if entry.CoverImageGCPKey != "" {
		_ = deleteGCSObjectHook(s.BucketName, entry.CoverImageGCPKey)
	}

	// Delete associated media from GCS
	var mediaList []PressMedia
	_ = s.DB.Where("press_entry_id = ?", id).Find(&mediaList)
	for _, media := range mediaList {
		if media.GCPObjectKey != "" {
			_ = deleteGCSObjectHook(s.BucketName, media.GCPObjectKey)
		}
	}

	// Delete from database (cascade will handle media deletion)
	if err := s.DB.Delete(&entry).Error; err != nil {
		return err
	}

	return nil
}

// AddPressMedia adds media files to a press entry
func (s *PressService) AddPressMedia(id int, req AddPressMediaRequest, userID *int) (*AddPressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	// Verify entry exists
	if err := s.DB.First(&PressEntry{}, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressEntryNotFound
		}
		return nil, err
	}

	if s.BucketName == "" {
		return nil, ErrMediaBucketNotConfigured
	}

	uploadedCount := 0
	for _, uploadInput := range req.Media {
		if len(uploadInput.Content) == 0 {
			continue
		}

		objectKey := path.Join(s.BucketPrefix, "press-entries", fmt.Sprintf("media-%d-%d", time.Now().UnixNano(), uploadInput.FileSize))
		fileURL, fileSize, err := uploadBytesToGCSHook(uploadInput.Content, s.BucketName, objectKey, uploadInput.MimeType)
		if err != nil {
			continue
		}

		media := PressMedia{
			PressEntryID: id,
			DisplayName:  uploadInput.DisplayName,
			FileName:     uploadInput.FileName,
			GCPObjectKey: objectKey,
			FileURL:      fileURL,
			MimeType:     uploadInput.MimeType,
			FileSize:     fileSize,
			MediaRole:    "attachment",
			SortOrder:    -1,
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}

		if err := s.DB.Create(&media).Error; err != nil {
			continue
		}

		uploadedCount++
	}

	// Update the press entry's updated_at
	if err := s.DB.Model(&PressEntry{}).Where("id = ?", id).Update("updated_by", userID).Error; err != nil {
		return nil, err
	}

	return &AddPressMediaResponse{UploadedCount: uploadedCount}, nil
}

// UpdatePressMedia updates metadata for a media file
func (s *PressService) UpdatePressMedia(id int, mediaID int, req UpdatePressMediaRequest) (*PressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var media PressMedia
	if err := s.DB.Where("id = ? AND press_entry_id = ?", mediaID, id).First(&media).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressMediaNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(req.DisplayName) != "" {
		media.DisplayName = req.DisplayName
	}

	if strings.TrimSpace(req.FileName) != "" {
		media.FileName = req.FileName
	}

	if err := s.DB.Save(&media).Error; err != nil {
		return nil, err
	}

	return &PressMediaResponse{
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
	}, nil
}

// ReorderPressMedia reorders media files
func (s *PressService) ReorderPressMedia(id int, mediaIDs []int) (*ReorderPressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	// Verify entry exists
	if err := s.DB.First(&PressEntry{}, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressEntryNotFound
		}
		return nil, err
	}

	// Reorder media based on provided IDs
	updatedCount := 0
	for index, mediaID := range mediaIDs {
		if err := s.DB.Model(&PressMedia{}).
			Where("id = ? AND press_entry_id = ?", mediaID, id).
			Update("sort_order", index).Error; err != nil {
			continue
		}
		updatedCount++
	}

	return &ReorderPressMediaResponse{UpdatedCount: updatedCount}, nil
}

// DeletePressMedia deletes media files from a press entry
func (s *PressService) DeletePressMedia(id int, mediaIDs []int) (*DeletePressMediaResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	// Verify entry exists
	if err := s.DB.First(&PressEntry{}, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPressEntryNotFound
		}
		return nil, err
	}

	// Get media files to delete
	var mediaList []PressMedia
	if err := s.DB.Where("id IN ? AND press_entry_id = ?", mediaIDs, id).Find(&mediaList).Error; err != nil {
		return nil, err
	}

	// Delete from GCS
	for _, media := range mediaList {
		if media.GCPObjectKey != "" {
			_ = deleteGCSObjectHook(s.BucketName, media.GCPObjectKey)
		}
	}

	// Delete from database
	if err := s.DB.Where("id IN ? AND press_entry_id = ?", mediaIDs, id).Delete(&PressMedia{}).Error; err != nil {
		return nil, err
	}

	return &DeletePressMediaResponse{DeletedCount: len(mediaList)}, nil
}

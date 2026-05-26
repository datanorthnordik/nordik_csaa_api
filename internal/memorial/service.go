package memorial

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrStoreUnavailable         = errors.New("memorial service unavailable")
	ErrMemorialNotFound         = errors.New("memorial entry not found")
	ErrMemorialMediaNotFound    = errors.New("memorial media not found")
	ErrMediaBucketNotConfigured = errors.New("media bucket is not configured")
)

var (
	memorialNowFunc      = time.Now
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

type MemorialService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

type storedObjectRef struct {
	ObjectKey string
	FileURL   string
}

type normalizedSaveMemorialRequest struct {
	FullName              string
	Affiliation           string
	Category              string
	Status                string
	Biography             string
	DateOfBirth           *time.Time
	DateOfPassing         *time.Time
	Portrait              *MemorialUploadInput
	RemovePortrait        bool
	GalleryImages         []MemorialUploadInput
	RemoveGalleryImageIDs []int
}

func (s *MemorialService) ListMemorials(filter ListMemorialsFilter) (*MemorialListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalizedFilter, err := normalizeListMemorialsFilter(filter)
	if err != nil {
		return nil, err
	}

	baseQuery := s.applyListFilters(s.DB.Model(&MemorialEntry{}), normalizedFilter, false)
	itemQuery := s.applyListFilters(s.DB.Model(&MemorialEntry{}), normalizedFilter, true)

	categoryCounts, err := s.listCategoryCounts(baseQuery)
	if err != nil {
		return nil, err
	}
	statusCounts, err := s.listStatusCounts(baseQuery)
	if err != nil {
		return nil, err
	}

	var totalItems int64
	if err := itemQuery.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	var rows []MemorialEntry
	if err := itemQuery.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "updated_at"}, Desc: true}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true}).
		Offset((normalizedFilter.Page - 1) * normalizedFilter.PageSize).
		Limit(normalizedFilter.PageSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]MemorialListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, memorialListItemFromModel(row))
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(normalizedFilter.PageSize) - 1) / int64(normalizedFilter.PageSize))
	}

	return &MemorialListResponse{
		Items: items,
		Pagination: MemorialListPageMeta{
			Page:       normalizedFilter.Page,
			PageSize:   normalizedFilter.PageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    normalizedFilter.Page < totalPages,
			HasPrev:    normalizedFilter.Page > 1 && totalPages > 0,
		},
		Summary: MemorialListSummary{
			CategoryCounts: categoryCounts,
			StatusCounts:   statusCounts,
		},
		Applied: MemorialListAppliedFilters{
			Page:       normalizedFilter.Page,
			PageSize:   normalizedFilter.PageSize,
			SearchTerm: normalizedFilter.SearchTerm,
			Status:     normalizedFilter.Status,
			Category:   normalizedFilter.Category,
		},
	}, nil
}

func (s *MemorialService) GetMemorial(id int) (*MemorialDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getMemorialEntryModel(id)
	if err != nil {
		return nil, err
	}

	var galleryRows []MemorialGalleryImage
	if err := s.DB.
		Where("memorial_entry_id = ?", id).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&galleryRows).Error; err != nil {
		return nil, err
	}

	resp := memorialDetailFromModel(entry, galleryRows)
	return &resp, nil
}

func (s *MemorialService) GetMemorialPortraitContent(id int) (*MemorialMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getMemorialEntryModel(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(entry.PortraitGCPObjectKey) == "" && strings.TrimSpace(entry.PortraitFileURL) == "" {
		return nil, ErrMemorialMediaNotFound
	}

	data, contentType, err := s.downloadStoredObject(storedObjectRef{
		ObjectKey: entry.PortraitGCPObjectKey,
		FileURL:   entry.PortraitFileURL,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = entry.PortraitMimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	fileName := strings.TrimSpace(entry.PortraitFileName)
	if fileName == "" {
		fileName = buildStoredFileName(entry.PortraitGCPObjectKey, "memorial-portrait")
	}

	return &MemorialMediaContent{
		Content:     data,
		ContentType: contentType,
		FileName:    fileName,
	}, nil
}

func (s *MemorialService) GetMemorialGalleryImageContent(id int, mediaID int) (*MemorialMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var image MemorialGalleryImage
	if err := s.DB.Where("memorial_entry_id = ? AND id = ?", id, mediaID).First(&image).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemorialMediaNotFound
		}
		return nil, err
	}

	data, contentType, err := s.downloadStoredObject(storedObjectRef{
		ObjectKey: image.GCPObjectKey,
		FileURL:   image.FileURL,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = image.MimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	fileName := strings.TrimSpace(image.FileName)
	if fileName == "" {
		fileName = buildStoredFileName(image.GCPObjectKey, "memorial-gallery-image")
	}

	return &MemorialMediaContent{
		Content:     data,
		ContentType: contentType,
		FileName:    fileName,
	}, nil
}

func (s *MemorialService) CreateMemorial(req SaveMemorialRequest, userID *int) (*MemorialMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, err := normalizeSaveMemorialRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	uploadedObjects := make([]string, 0, 1+len(cleanReq.GalleryImages))

	entry := MemorialEntry{
		FullName:      cleanReq.FullName,
		Affiliation:   cleanReq.Affiliation,
		Category:      cleanReq.Category,
		Status:        cleanReq.Status,
		Biography:     cleanReq.Biography,
		DateOfBirth:   cleanReq.DateOfBirth,
		DateOfPassing: cleanReq.DateOfPassing,
		PublishedAt:   nextPublishedAt(nil, cleanReq.Status),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	if err := tx.
		Omit("PortraitFileName", "PortraitGCPObjectKey", "PortraitFileURL", "PortraitMimeType", "PortraitFileSize").
		Create(&entry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if cleanReq.Portrait != nil {
		fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := s.storeMemorialImage(entry.ID, "portrait", 0, *cleanReq.Portrait)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedKey != "" {
			uploadedObjects = append(uploadedObjects, uploadedKey)
		}
		entry.PortraitFileURL = fileURL
		entry.PortraitGCPObjectKey = objectKey
		entry.PortraitFileName = fileName
		entry.PortraitMimeType = mimeType
		entry.PortraitFileSize = fileSize
	}

	if len(cleanReq.GalleryImages) > 0 {
		for idx, input := range cleanReq.GalleryImages {
			fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := s.storeMemorialImage(entry.ID, "gallery", idx, input)
			if err != nil {
				tx.Rollback()
				s.cleanupObjects(uploadedObjects)
				return nil, err
			}
			if uploadedKey != "" {
				uploadedObjects = append(uploadedObjects, uploadedKey)
			}
			row := MemorialGalleryImage{
				MemorialEntryID: entry.ID,
				FileName:        fileName,
				GCPObjectKey:    objectKey,
				FileURL:         fileURL,
				MimeType:        mimeType,
				FileSize:        fileSize,
				SortOrder:       idx,
				UploadedBy:      userID,
			}
			if err := tx.Create(&row).Error; err != nil {
				tx.Rollback()
				s.cleanupObjects(uploadedObjects)
				return nil, err
			}
		}
	}

	if cleanReq.Portrait != nil {
		if err := persistMemorialEntry(tx, entry); err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return memorialMutationFromModel(entry), nil
}

func (s *MemorialService) UpdateMemorial(id int, req SaveMemorialRequest, userID *int) (*MemorialMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, err := normalizeSaveMemorialRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	var entry MemorialEntry
	if err := tx.First(&entry, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemorialNotFound
		}
		return nil, err
	}

	uploadedObjects := make([]string, 0, 1+len(cleanReq.GalleryImages))
	oldObjects := make([]storedObjectRef, 0, 1+len(cleanReq.RemoveGalleryImageIDs))

	entry.FullName = cleanReq.FullName
	entry.Affiliation = cleanReq.Affiliation
	entry.Category = cleanReq.Category
	entry.Status = cleanReq.Status
	entry.Biography = cleanReq.Biography
	entry.DateOfBirth = cleanReq.DateOfBirth
	entry.DateOfPassing = cleanReq.DateOfPassing
	entry.PublishedAt = nextPublishedAt(entry.PublishedAt, cleanReq.Status)
	entry.UpdatedBy = userID

	if cleanReq.RemovePortrait && (strings.TrimSpace(entry.PortraitGCPObjectKey) != "" || strings.TrimSpace(entry.PortraitFileURL) != "") {
		oldObjects = append(oldObjects, storedObjectRef{
			ObjectKey: entry.PortraitGCPObjectKey,
			FileURL:   entry.PortraitFileURL,
		})
		entry.PortraitFileName = ""
		entry.PortraitGCPObjectKey = ""
		entry.PortraitFileURL = ""
		entry.PortraitMimeType = ""
		entry.PortraitFileSize = 0
	}

	if cleanReq.Portrait != nil {
		if strings.TrimSpace(entry.PortraitGCPObjectKey) != "" || strings.TrimSpace(entry.PortraitFileURL) != "" {
			oldObjects = append(oldObjects, storedObjectRef{
				ObjectKey: entry.PortraitGCPObjectKey,
				FileURL:   entry.PortraitFileURL,
			})
		}
		fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := s.storeMemorialImage(entry.ID, "portrait", 0, *cleanReq.Portrait)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedKey != "" {
			uploadedObjects = append(uploadedObjects, uploadedKey)
		}
		entry.PortraitFileURL = fileURL
		entry.PortraitGCPObjectKey = objectKey
		entry.PortraitFileName = fileName
		entry.PortraitMimeType = mimeType
		entry.PortraitFileSize = fileSize
	}

	if len(cleanReq.RemoveGalleryImageIDs) > 0 {
		var rows []MemorialGalleryImage
		if err := tx.
			Where("memorial_entry_id = ? AND id IN ?", id, cleanReq.RemoveGalleryImageIDs).
			Find(&rows).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if len(rows) > 0 {
			for _, row := range rows {
				oldObjects = append(oldObjects, storedObjectRef{
					ObjectKey: row.GCPObjectKey,
					FileURL:   row.FileURL,
				})
			}
			if err := tx.Delete(&rows).Error; err != nil {
				tx.Rollback()
				s.cleanupObjects(uploadedObjects)
				return nil, err
			}
			if err := s.resequenceGalleryImages(tx, id); err != nil {
				tx.Rollback()
				s.cleanupObjects(uploadedObjects)
				return nil, err
			}
		}
	}

	if len(cleanReq.GalleryImages) > 0 {
		startSortOrder, err := s.nextGallerySortOrder(tx, id)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		for idx, input := range cleanReq.GalleryImages {
			fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := s.storeMemorialImage(entry.ID, "gallery", idx, input)
			if err != nil {
				tx.Rollback()
				s.cleanupObjects(uploadedObjects)
				return nil, err
			}
			if uploadedKey != "" {
				uploadedObjects = append(uploadedObjects, uploadedKey)
			}
			row := MemorialGalleryImage{
				MemorialEntryID: id,
				FileName:        fileName,
				GCPObjectKey:    objectKey,
				FileURL:         fileURL,
				MimeType:        mimeType,
				FileSize:        fileSize,
				SortOrder:       startSortOrder + idx,
				UploadedBy:      userID,
			}
			if err := tx.Create(&row).Error; err != nil {
				tx.Rollback()
				s.cleanupObjects(uploadedObjects)
				return nil, err
			}
		}
	}

	if err := persistMemorialEntry(tx, entry); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	s.cleanupStoredObjectsBestEffort(oldObjects)
	return memorialMutationFromModel(entry), nil
}

func (s *MemorialService) DeleteMemorial(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackOnPanic(tx)

	var entry MemorialEntry
	if err := tx.First(&entry, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemorialNotFound
		}
		return err
	}

	var galleryRows []MemorialGalleryImage
	if err := tx.Where("memorial_entry_id = ?", id).Find(&galleryRows).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Delete(&entry).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	toCleanup := make([]storedObjectRef, 0, len(galleryRows)+1)
	if strings.TrimSpace(entry.PortraitGCPObjectKey) != "" || strings.TrimSpace(entry.PortraitFileURL) != "" {
		toCleanup = append(toCleanup, storedObjectRef{
			ObjectKey: entry.PortraitGCPObjectKey,
			FileURL:   entry.PortraitFileURL,
		})
	}
	for _, row := range galleryRows {
		toCleanup = append(toCleanup, storedObjectRef{
			ObjectKey: row.GCPObjectKey,
			FileURL:   row.FileURL,
		})
	}
	s.cleanupStoredObjectsBestEffort(toCleanup)
	return nil
}

func normalizeListMemorialsFilter(filter ListMemorialsFilter) (ListMemorialsFilter, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 10
	}

	filter.SearchTerm = strings.TrimSpace(filter.SearchTerm)
	filter.Status = normalizeMemorialStatusFilter(filter.Status)
	filter.Category = normalizeMemorialCategory(filter.Category)

	if !isAllowedMemorialStatusFilter(filter.Status) {
		return filter, fmt.Errorf("status must be one of all, draft, review, published")
	}
	if filter.Category != "" && !isAllowedMemorialCategory(filter.Category) {
		return filter, fmt.Errorf("category must be one of alumnus, veteran, founder, friend")
	}

	return filter, nil
}

func normalizeSaveMemorialRequest(req SaveMemorialRequest) (normalizedSaveMemorialRequest, error) {
	var normalized normalizedSaveMemorialRequest

	normalized.FullName = strings.TrimSpace(req.FullName)
	normalized.Affiliation = strings.TrimSpace(req.Affiliation)
	normalized.Category = normalizeMemorialCategory(req.Category)
	normalized.Status = normalizeMemorialStatus(req.Status)
	normalized.Biography = strings.TrimSpace(req.Biography)
	normalized.RemovePortrait = req.RemovePortrait
	normalized.RemoveGalleryImageIDs = uniquePositiveInts(req.RemoveGalleryImageIDs)

	if normalized.FullName == "" {
		return normalized, fmt.Errorf("full_name is required")
	}
	if len(normalized.FullName) > 255 {
		return normalized, fmt.Errorf("full_name must be 255 characters or fewer")
	}
	if normalized.Affiliation != "" && len(normalized.Affiliation) > 255 {
		return normalized, fmt.Errorf("affiliation must be 255 characters or fewer")
	}
	if !isAllowedMemorialCategory(normalized.Category) {
		return normalized, fmt.Errorf("category must be one of alumnus, veteran, founder, friend")
	}
	if !isAllowedMemorialStatus(normalized.Status) {
		return normalized, fmt.Errorf("status must be one of draft, review, published")
	}

	dob, err := parseOptionalISODate(req.DateOfBirth)
	if err != nil {
		return normalized, fmt.Errorf("date_of_birth must be a valid date in YYYY-MM-DD format")
	}
	dop, err := parseOptionalISODate(req.DateOfPassing)
	if err != nil {
		return normalized, fmt.Errorf("date_of_passing must be a valid date in YYYY-MM-DD format")
	}
	if dob != nil && dop != nil && dop.Before(*dob) {
		return normalized, fmt.Errorf("date_of_passing must be on or after date_of_birth")
	}
	normalized.DateOfBirth = dob
	normalized.DateOfPassing = dop

	if req.Portrait != nil {
		cleaned := sanitizeUploadInput(*req.Portrait)
		if err := validateImageUploadInput(cleaned); err != nil {
			return normalized, err
		}
		normalized.Portrait = &cleaned
	}

	normalized.GalleryImages = make([]MemorialUploadInput, 0, len(req.GalleryImages))
	for _, image := range req.GalleryImages {
		cleaned := sanitizeUploadInput(image)
		if err := validateImageUploadInput(cleaned); err != nil {
			return normalized, err
		}
		normalized.GalleryImages = append(normalized.GalleryImages, cleaned)
	}

	return normalized, nil
}

func (s *MemorialService) applyListFilters(query *gorm.DB, filter ListMemorialsFilter, includeTaxonomy bool) *gorm.DB {
	if searchTerm := strings.TrimSpace(filter.SearchTerm); searchTerm != "" {
		pattern := "%" + strings.ToLower(searchTerm) + "%"
		query = query.Where(
			"LOWER(COALESCE(full_name, '')) LIKE ? OR LOWER(COALESCE(affiliation, '')) LIKE ? OR LOWER(COALESCE(biography, '')) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if includeTaxonomy {
		if filter.Status != "" && filter.Status != "all" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.Category != "" {
			query = query.Where("category = ?", filter.Category)
		}
	}

	return query
}

func (s *MemorialService) listCategoryCounts(query *gorm.DB) ([]MemorialCategoryCount, error) {
	type countRow struct {
		Category string
		Count    int64
	}

	var rows []countRow
	if err := query.
		Select("category, COUNT(*) AS count").
		Group("category").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	countMap := make(map[string]int64, len(rows))
	for _, row := range rows {
		countMap[normalizeMemorialCategory(row.Category)] = row.Count
	}

	counts := make([]MemorialCategoryCount, 0, len(memorialCategoryOrder))
	for _, category := range memorialCategoryOrder {
		counts = append(counts, MemorialCategoryCount{
			Category: category,
			Label:    memorialCategoryLabel(category),
			Count:    countMap[category],
		})
	}

	return counts, nil
}

func (s *MemorialService) listStatusCounts(query *gorm.DB) ([]MemorialStatusCount, error) {
	type countRow struct {
		Status string
		Count  int64
	}

	var rows []countRow
	if err := query.
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	countMap := make(map[string]int64, len(rows))
	for _, row := range rows {
		countMap[normalizeMemorialStatus(row.Status)] = row.Count
	}

	counts := make([]MemorialStatusCount, 0, len(memorialStatusOrder))
	for _, status := range memorialStatusOrder {
		counts = append(counts, MemorialStatusCount{
			Status: status,
			Label:  memorialStatusLabel(status),
			Count:  countMap[status],
		})
	}

	return counts, nil
}

func (s *MemorialService) getMemorialEntryModel(id int) (MemorialEntry, error) {
	var entry MemorialEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MemorialEntry{}, ErrMemorialNotFound
		}
		return MemorialEntry{}, err
	}
	return entry, nil
}

func memorialListItemFromModel(entry MemorialEntry) MemorialListItem {
	item := MemorialListItem{
		ID:            entry.ID,
		FullName:      entry.FullName,
		Affiliation:   entry.Affiliation,
		Category:      entry.Category,
		CategoryLabel: memorialCategoryLabel(entry.Category),
		Status:        entry.Status,
		DateOfBirth:   formatOptionalDate(entry.DateOfBirth),
		DateOfPassing: formatOptionalDate(entry.DateOfPassing),
		CreatedAt:     entry.CreatedAt,
		UpdatedAt:     entry.UpdatedAt,
		PublishedAt:   cloneTimePointer(entry.PublishedAt),
	}
	if strings.TrimSpace(entry.PortraitFileURL) != "" || strings.TrimSpace(entry.PortraitGCPObjectKey) != "" {
		item.PortraitContentURL = buildMemorialPortraitContentURL(entry.ID)
	}
	return item
}

func memorialDetailFromModel(entry MemorialEntry, galleryRows []MemorialGalleryImage) MemorialDetailResponse {
	resp := MemorialDetailResponse{
		ID:            entry.ID,
		FullName:      entry.FullName,
		Affiliation:   entry.Affiliation,
		Category:      entry.Category,
		CategoryLabel: memorialCategoryLabel(entry.Category),
		Status:        entry.Status,
		Biography:     entry.Biography,
		DateOfBirth:   formatOptionalDate(entry.DateOfBirth),
		DateOfPassing: formatOptionalDate(entry.DateOfPassing),
		GalleryImages: make([]MemorialGalleryImageResponse, 0, len(galleryRows)),
		CreatedAt:     entry.CreatedAt,
		UpdatedAt:     entry.UpdatedAt,
		PublishedAt:   cloneTimePointer(entry.PublishedAt),
	}

	if strings.TrimSpace(entry.PortraitFileURL) != "" || strings.TrimSpace(entry.PortraitGCPObjectKey) != "" {
		resp.Portrait = &MemorialMediaResponse{
			FileName:   entry.PortraitFileName,
			MimeType:   entry.PortraitMimeType,
			FileSize:   entry.PortraitFileSize,
			ContentURL: buildMemorialPortraitContentURL(entry.ID),
			CreatedAt:  entry.UpdatedAt,
		}
	}

	for _, row := range galleryRows {
		resp.GalleryImages = append(resp.GalleryImages, MemorialGalleryImageResponse{
			ID:         row.ID,
			FileName:   row.FileName,
			MimeType:   row.MimeType,
			FileSize:   row.FileSize,
			SortOrder:  row.SortOrder,
			ContentURL: buildMemorialGalleryImageContentURL(row.MemorialEntryID, row.ID),
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		})
	}

	return resp
}

func memorialMutationFromModel(entry MemorialEntry) *MemorialMutationResponse {
	return &MemorialMutationResponse{
		ID:        entry.ID,
		FullName:  entry.FullName,
		Category:  entry.Category,
		Status:    entry.Status,
		UpdatedAt: entry.UpdatedAt,
	}
}

func buildMemorialPortraitContentURL(id int) string {
	return fmt.Sprintf("/api/memorial/%d/portrait/content", id)
}

func buildMemorialGalleryImageContentURL(memorialID int, imageID int) string {
	return fmt.Sprintf("/api/memorial/%d/gallery/%d/content", memorialID, imageID)
}

func parseOptionalISODate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func formatOptionalDate(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func sanitizeUploadInput(input MemorialUploadInput) MemorialUploadInput {
	input.FileName = strings.TrimSpace(input.FileName)
	input.MimeType = strings.TrimSpace(input.MimeType)
	input.FileURL = strings.TrimSpace(input.FileURL)
	input.GCPObjectKey = strings.TrimSpace(input.GCPObjectKey)
	return input
}

func validateImageUploadInput(input MemorialUploadInput) error {
	if len(input.Content) == 0 && strings.TrimSpace(input.FileURL) == "" && strings.TrimSpace(input.GCPObjectKey) == "" {
		return fmt.Errorf("image file is required")
	}

	mimeType := strings.ToLower(strings.TrimSpace(input.MimeType))
	if mimeType != "" && !strings.HasPrefix(mimeType, "image/") {
		return fmt.Errorf("only image uploads are supported")
	}

	if mimeType == "" {
		ext := strings.ToLower(util.ExtFromFilenameOrMime(input.FileName, input.MimeType))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		default:
			if len(input.Content) > 0 {
				return fmt.Errorf("only image uploads are supported")
			}
		}
	}

	return nil
}

func uniquePositiveInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[int]struct{}, len(values))
	cleaned := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	sort.Ints(cleaned)
	return cleaned
}

func normalizeMemorialCategory(category string) string {
	return strings.ToLower(strings.TrimSpace(category))
}

func isAllowedMemorialCategory(category string) bool {
	switch normalizeMemorialCategory(category) {
	case MemorialCategoryAlumnus,
		MemorialCategoryVeteran,
		MemorialCategoryFounder,
		MemorialCategoryFriend:
		return true
	default:
		return false
	}
}

func memorialCategoryLabel(category string) string {
	switch normalizeMemorialCategory(category) {
	case MemorialCategoryAlumnus:
		return "Alumnus"
	case MemorialCategoryVeteran:
		return "Veteran"
	case MemorialCategoryFounder:
		return "Founder"
	case MemorialCategoryFriend:
		return "Friend"
	default:
		return "Memorial"
	}
}

func normalizeMemorialStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func normalizeMemorialStatusFilter(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "all"
	}
	return status
}

func isAllowedMemorialStatus(status string) bool {
	switch normalizeMemorialStatus(status) {
	case MemorialStatusDraft, MemorialStatusReview, MemorialStatusPublished:
		return true
	default:
		return false
	}
}

func isAllowedMemorialStatusFilter(status string) bool {
	switch normalizeMemorialStatusFilter(status) {
	case "all", MemorialStatusDraft, MemorialStatusReview, MemorialStatusPublished:
		return true
	default:
		return false
	}
}

func memorialStatusLabel(status string) string {
	switch normalizeMemorialStatus(status) {
	case MemorialStatusPublished:
		return "Published"
	case MemorialStatusReview:
		return "Under Review"
	case MemorialStatusDraft:
		return "Draft"
	default:
		return "Draft"
	}
}

var memorialCategoryOrder = []string{
	MemorialCategoryAlumnus,
	MemorialCategoryVeteran,
	MemorialCategoryFounder,
	MemorialCategoryFriend,
}

var memorialStatusOrder = []string{
	MemorialStatusPublished,
	MemorialStatusDraft,
	MemorialStatusReview,
}

func nextPublishedAt(current *time.Time, status string) *time.Time {
	if normalizeMemorialStatus(status) != MemorialStatusPublished {
		return nil
	}
	if current != nil && !current.IsZero() {
		return cloneTimePointer(current)
	}
	now := memorialNowFunc()
	return &now
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (s *MemorialService) storeMemorialImage(memorialID int, folder string, idx int, input MemorialUploadInput) (fileURL string, objectKey string, fileName string, mimeType string, fileSize int64, uploadedKey string, err error) {
	fileName = strings.TrimSpace(input.FileName)
	mimeType = strings.TrimSpace(input.MimeType)
	fileURL = strings.TrimSpace(input.FileURL)
	objectKey = strings.TrimSpace(input.GCPObjectKey)
	fileSize = input.FileSize

	if len(input.Content) == 0 {
		if fileURL == "" && objectKey == "" {
			return "", "", "", "", 0, "", fmt.Errorf("image file is required")
		}
		if fileName == "" {
			fileName = buildStoredFileName(objectKey, "memorial-image")
		}
		if objectKey == "" && looksLikeGCSReference(fileURL) {
			if _, resolvedObjectKey, parseErr := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL); parseErr == nil {
				objectKey = resolvedObjectKey
			}
		}
		return fileURL, objectKey, fileName, mimeType, fileSize, "", nil
	}

	if strings.TrimSpace(s.BucketName) == "" {
		return "", "", "", "", 0, "", ErrMediaBucketNotConfigured
	}

	if mimeType == "" {
		mimeType = http.DetectContentType(input.Content)
	}
	if fileName == "" {
		fileName = "memorial-image" + util.ExtFromFilenameOrMime(fileName, mimeType)
	}

	objectKey = s.buildObjectKey(memorialID, folder, idx, fileName, mimeType)
	uploadedURL, uploadedSize, uploadErr := uploadBytesToGCSHook(input.Content, s.BucketName, objectKey, mimeType)
	if uploadErr != nil {
		return "", "", "", "", 0, "", uploadErr
	}
	if strings.TrimSpace(uploadedURL) == "" {
		return "", objectKey, "", "", 0, objectKey, fmt.Errorf("memorial upload returned an empty file URL")
	}

	return uploadedURL, objectKey, fileName, mimeType, uploadedSize, objectKey, nil
}

func (s *MemorialService) buildObjectKey(memorialID int, folder string, idx int, fileName string, mimeType string) string {
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	timestamp := memorialNowFunc().UTC().Format("20060102150405")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = folder
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)

	objectName := fmt.Sprintf("%s_%02d_%s%s", timestamp, idx+1, base, ext)
	key := path.Join("memorial", fmt.Sprintf("entry-%d", memorialID), folder, objectName)
	if prefix == "" {
		return key
	}
	return path.Join(prefix, key)
}

func (s *MemorialService) resolveStoredObjectReference(objectKey string, fileURL string) (string, string, error) {
	objectKey = strings.TrimSpace(objectKey)
	fileURL = strings.TrimSpace(fileURL)

	if objectKey != "" && strings.TrimSpace(s.BucketName) != "" {
		return strings.TrimSpace(s.BucketName), objectKey, nil
	}
	if fileURL == "" {
		if objectKey != "" {
			return "", "", ErrMediaBucketNotConfigured
		}
		return "", "", fmt.Errorf("memorial content is not available from storage")
	}
	if !looksLikeGCSReference(fileURL) {
		return "", "", fmt.Errorf("memorial content is not available from storage")
	}

	bucketName, resolvedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL)
	if err != nil {
		if errors.Is(err, util.ErrBucketNameRequired) {
			return "", "", ErrMediaBucketNotConfigured
		}
		if errors.Is(err, util.ErrObjectNameRequired) {
			return "", "", fmt.Errorf("memorial content is not available from storage")
		}
		return "", "", err
	}
	if strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		return "", "", fmt.Errorf("memorial content is not available from storage")
	}
	return bucketName, resolvedObjectKey, nil
}

func (s *MemorialService) downloadStoredObject(ref storedObjectRef) ([]byte, string, error) {
	bucketName, objectKey, err := s.resolveStoredObjectReference(ref.ObjectKey, ref.FileURL)
	if err != nil {
		return nil, "", err
	}

	data, contentType, err := downloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, "", ErrMemorialMediaNotFound
		}
		return nil, "", err
	}
	return data, contentType, nil
}

func (s *MemorialService) cleanupObjects(objectKeys []string) {
	for _, objectKey := range objectKeys {
		objectKey = strings.TrimSpace(objectKey)
		if objectKey == "" || strings.TrimSpace(s.BucketName) == "" {
			continue
		}
		_ = deleteGCSObjectHook(s.BucketName, objectKey)
	}
}

func (s *MemorialService) cleanupStoredObjectsBestEffort(items []storedObjectRef) {
	for _, item := range items {
		bucketName, objectKey, err := s.resolveStoredObjectReference(item.ObjectKey, item.FileURL)
		if err != nil || strings.TrimSpace(bucketName) == "" || strings.TrimSpace(objectKey) == "" {
			continue
		}
		_ = deleteGCSObjectHook(bucketName, objectKey)
	}
}

func (s *MemorialService) nextGallerySortOrder(tx *gorm.DB, memorialID int) (int, error) {
	type maxSortOrderRow struct {
		MaxSortOrder int `gorm:"column:max_sort_order"`
	}

	var row maxSortOrderRow
	if err := tx.Model(&MemorialGalleryImage{}).
		Select("COALESCE(MAX(sort_order), -1) AS max_sort_order").
		Where("memorial_entry_id = ?", memorialID).
		Scan(&row).Error; err != nil {
		return 0, err
	}

	return row.MaxSortOrder + 1, nil
}

func (s *MemorialService) resequenceGalleryImages(tx *gorm.DB, memorialID int) error {
	var rows []MemorialGalleryImage
	if err := tx.
		Where("memorial_entry_id = ?", memorialID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return err
	}

	for idx, row := range rows {
		if row.SortOrder == idx {
			continue
		}
		if err := tx.Model(&MemorialGalleryImage{}).
			Where("memorial_entry_id = ? AND id = ?", memorialID, row.ID).
			Update("sort_order", idx).Error; err != nil {
			return err
		}
	}

	return nil
}

func buildStoredFileName(objectKey string, fallback string) string {
	base := path.Base(strings.TrimSpace(objectKey))
	if base == "" || base == "." || base == "/" {
		return fallback
	}
	return base
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

func rollbackOnPanic(tx *gorm.DB) {
	if recover() != nil {
		tx.Rollback()
		panic("transaction panic")
	}
}

func persistMemorialEntry(tx *gorm.DB, entry MemorialEntry) error {
	updates := map[string]interface{}{
		"full_name":               entry.FullName,
		"affiliation":             entry.Affiliation,
		"category":                entry.Category,
		"status":                  entry.Status,
		"biography":               entry.Biography,
		"date_of_birth":           entry.DateOfBirth,
		"date_of_passing":         entry.DateOfPassing,
		"published_at":            entry.PublishedAt,
		"updated_by":              entry.UpdatedBy,
		"portrait_file_name":      nullableMemorialString(entry.PortraitFileName),
		"portrait_gcp_object_key": nullableMemorialString(entry.PortraitGCPObjectKey),
		"portrait_file_url":       nullableMemorialString(entry.PortraitFileURL),
		"portrait_mime_type":      nullableMemorialString(entry.PortraitMimeType),
		"portrait_file_size":      nullableMemorialInt64(entry.PortraitFileSize),
	}

	return tx.Model(&MemorialEntry{}).Where("id = ?", entry.ID).Updates(updates).Error
}

func nullableMemorialString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableMemorialInt64(value int64) interface{} {
	if value <= 0 {
		return nil
	}
	return value
}

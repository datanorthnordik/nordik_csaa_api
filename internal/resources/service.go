package resources

import (
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
	ErrStoreUnavailable         = errors.New("resource service unavailable")
	ErrResourceNotFound         = errors.New("resource not found")
	ErrMediaBucketNotConfigured = errors.New("media bucket is not configured")
)

var (
	resourceNowFunc      = time.Now
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

type ResourceService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

func (s *ResourceService) ListResources(filter ListResourcesFilter) (*ResourceListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalizedFilter, err := normalizeListResourcesFilter(filter)
	if err != nil {
		return nil, err
	}

	baseQuery, err := s.applyListFilters(s.DB.Model(&ResourceEntry{}), normalizedFilter, false)
	if err != nil {
		return nil, err
	}
	itemQuery, err := s.applyListFilters(s.DB.Model(&ResourceEntry{}), normalizedFilter, true)
	if err != nil {
		return nil, err
	}

	categoryCounts, err := s.listCategoryCounts(baseQuery)
	if err != nil {
		return nil, err
	}

	var totalItems int64
	if err := itemQuery.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	var rows []ResourceEntry
	if err := itemQuery.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "updated_at"}, Desc: true}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true}).
		Offset((normalizedFilter.Page - 1) * normalizedFilter.PageSize).
		Limit(normalizedFilter.PageSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]ResourceListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, resourceListItemFromModel(row))
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(normalizedFilter.PageSize) - 1) / int64(normalizedFilter.PageSize))
	}

	return &ResourceListResponse{
		Items: items,
		Pagination: ResourceListPageMeta{
			Page:       normalizedFilter.Page,
			PageSize:   normalizedFilter.PageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    normalizedFilter.Page < totalPages,
			HasPrev:    normalizedFilter.Page > 1 && totalPages > 0,
		},
		Summary: ResourceListSummary{
			CategoryCounts: categoryCounts,
		},
		Applied: ResourceListAppliedFilters{
			Page:       normalizedFilter.Page,
			PageSize:   normalizedFilter.PageSize,
			SearchTerm: normalizedFilter.SearchTerm,
			Category:   normalizedFilter.Category,
			FileType:   normalizedFilter.FileType,
		},
	}, nil
}

func (s *ResourceService) GetResource(id int) (*ResourceDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getResourceEntryModel(id)
	if err != nil {
		return nil, err
	}

	resp := ResourceDetailResponse(resourceListItemFromModel(entry))
	return &resp, nil
}

func (s *ResourceService) GetResourceContent(id int) (*ResourceContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getResourceEntryModel(id)
	if err != nil {
		return nil, err
	}

	bucketName, objectKey, err := s.resolveStoredObjectReference(entry.GCPObjectKey, entry.FileURL)
	if err != nil {
		return nil, err
	}

	data, contentType, err := downloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = entry.MimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return &ResourceContent{
		Content:     data,
		ContentType: contentType,
		FileName:    entry.FileName,
	}, nil
}

func (s *ResourceService) CreateResource(req SaveResourceRequest, userID *int) (*ResourceMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, err := normalizeSaveResourceRequest(req, true)
	if err != nil {
		return nil, err
	}

	fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := s.resolveDocument(cleanReq.Document, userID)
	if err != nil {
		return nil, err
	}

	entry := ResourceEntry{
		Name:         cleanReq.Name,
		Category:     cleanReq.Category,
		Visibility:   cleanReq.Visibility,
		FileName:     fileName,
		GCPObjectKey: objectKey,
		FileURL:      fileURL,
		MimeType:     mimeType,
		FileSize:     fileSize,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}

	if err := s.DB.Create(&entry).Error; err != nil {
		s.deleteObjectBestEffort(uploadedKey)
		return nil, err
	}

	return resourceMutationFromModel(entry), nil
}

func (s *ResourceService) UpdateResource(id int, req SaveResourceRequest, userID *int) (*ResourceMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getResourceEntryModel(id)
	if err != nil {
		return nil, err
	}

	cleanReq, err := normalizeSaveResourceRequest(req, false)
	if err != nil {
		return nil, err
	}

	oldObjectKey := entry.GCPObjectKey
	oldFileURL := entry.FileURL
	newUploadedKey := ""

	entry.Name = cleanReq.Name
	entry.Category = cleanReq.Category
	entry.Visibility = cleanReq.Visibility
	entry.UpdatedBy = userID

	if cleanReq.Document != nil {
		fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := s.resolveDocument(cleanReq.Document, userID)
		if err != nil {
			return nil, err
		}
		entry.FileURL = fileURL
		entry.GCPObjectKey = objectKey
		entry.FileName = fileName
		entry.MimeType = mimeType
		entry.FileSize = fileSize
		newUploadedKey = uploadedKey
	}

	if err := s.DB.Save(&entry).Error; err != nil {
		s.deleteObjectBestEffort(newUploadedKey)
		return nil, err
	}

	if newUploadedKey != "" && (strings.TrimSpace(oldObjectKey) != "" || strings.TrimSpace(oldFileURL) != "") {
		s.deleteStoredObjectBestEffort(oldObjectKey, oldFileURL)
	}

	return resourceMutationFromModel(entry), nil
}

func (s *ResourceService) DeleteResource(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	entry, err := s.getResourceEntryModel(id)
	if err != nil {
		return err
	}

	if err := s.DB.Delete(&entry).Error; err != nil {
		return err
	}

	s.deleteStoredObjectBestEffort(entry.GCPObjectKey, entry.FileURL)
	return nil
}

func normalizeListResourcesFilter(filter ListResourcesFilter) (ListResourcesFilter, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 10
	}
	filter.SearchTerm = strings.TrimSpace(filter.SearchTerm)
	filter.Category = normalizeResourceCategory(filter.Category)
	filter.FileType = normalizeResourceFileType(filter.FileType)

	if filter.Category != "" && !isAllowedResourceCategory(filter.Category) {
		return filter, fmt.Errorf("category must be one of brand_identity, governance_legal, training_manuals, media_kits")
	}
	if filter.FileType == "" {
		filter.FileType = "all"
	}
	if !isAllowedResourceFileType(filter.FileType) {
		return filter, fmt.Errorf("file_type is invalid")
	}

	return filter, nil
}

func normalizeSaveResourceRequest(req SaveResourceRequest, requireDocument bool) (SaveResourceRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Category = normalizeResourceCategory(req.Category)
	req.Visibility = strings.ToLower(strings.TrimSpace(req.Visibility))

	if req.Name == "" {
		return req, fmt.Errorf("name is required")
	}
	if len(req.Name) > 255 {
		return req, fmt.Errorf("name must be 255 characters or fewer")
	}
	if !isAllowedResourceCategory(req.Category) {
		return req, fmt.Errorf("category must be one of brand_identity, governance_legal, training_manuals, media_kits")
	}
	if req.Visibility == "" {
		req.Visibility = ResourceVisibilityPublic
	}
	if !isAllowedResourceVisibility(req.Visibility) {
		return req, fmt.Errorf("visibility must be one of public, internal")
	}
	if requireDocument {
		if req.Document == nil {
			return req, fmt.Errorf("document is required")
		}
		if len(req.Document.Content) == 0 &&
			strings.TrimSpace(req.Document.FileURL) == "" &&
			strings.TrimSpace(req.Document.GCPObjectKey) == "" {
			return req, fmt.Errorf("document file is required")
		}
	}
	return req, nil
}

func (s *ResourceService) resolveDocument(input *ResourceUploadInput, userID *int) (fileURL string, objectKey string, fileName string, mimeType string, fileSize int64, uploadedKey string, err error) {
	if input == nil {
		return "", "", "", "", 0, "", fmt.Errorf("document is required")
	}

	fileName = sanitizeStoredFilename(input.FileName)
	mimeType = normalizeMimeType(input.MimeType)
	fileURL = strings.TrimSpace(input.FileURL)
	objectKey = strings.TrimSpace(input.GCPObjectKey)
	fileSize = input.FileSize

	if len(input.Content) > 0 {
		if strings.TrimSpace(s.BucketName) == "" {
			return "", "", "", "", 0, "", ErrMediaBucketNotConfigured
		}

		objectKey = s.buildObjectKey(fileName, userID)
		uploadedURL, uploadedSize, uploadErr := uploadBytesToGCSHook(input.Content, s.BucketName, objectKey, mimeType)
		if uploadErr != nil {
			return "", "", "", "", 0, "", uploadErr
		}
		if strings.TrimSpace(uploadedURL) == "" {
			return "", objectKey, "", "", 0, objectKey, fmt.Errorf("resource upload returned an empty file URL")
		}
		fileURL = uploadedURL
		fileSize = uploadedSize
		uploadedKey = objectKey
	}

	if fileURL == "" {
		return "", uploadedKey, "", "", 0, uploadedKey, fmt.Errorf("document file is required")
	}
	if fileName == "" {
		fileName = storedFilename(objectKey, "resource-file")
	}
	if objectKey == "" && looksLikeGCSReference(fileURL) {
		if resolvedBucket, resolvedObjectKey, parseErr := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL); parseErr == nil && strings.TrimSpace(resolvedBucket) != "" {
			objectKey = resolvedObjectKey
		}
	}

	return fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, nil
}

func (s *ResourceService) applyListFilters(query *gorm.DB, filter ListResourcesFilter, includeCategory bool) (*gorm.DB, error) {
	if searchTerm := strings.TrimSpace(filter.SearchTerm); searchTerm != "" {
		pattern := "%" + strings.ToLower(searchTerm) + "%"
		query = query.Where(
			"LOWER(COALESCE(name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if includeCategory && strings.TrimSpace(filter.Category) != "" {
		query = query.Where("category = ?", filter.Category)
	}

	if filter.FileType != "" && filter.FileType != "all" {
		condition, args, err := resourceFileTypeCondition(filter.FileType)
		if err != nil {
			return nil, err
		}
		query = query.Where(condition, args...)
	}

	return query, nil
}

func (s *ResourceService) listCategoryCounts(query *gorm.DB) ([]ResourceCategoryCount, error) {
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
		countMap[normalizeResourceCategory(row.Category)] = row.Count
	}

	categoryCounts := make([]ResourceCategoryCount, 0, len(resourceCategoryOrder))
	for _, category := range resourceCategoryOrder {
		categoryCounts = append(categoryCounts, ResourceCategoryCount{
			Category: category,
			Label:    resourceCategoryLabel(category),
			Count:    countMap[category],
		})
	}

	return categoryCounts, nil
}

func (s *ResourceService) buildObjectKey(fileName string, userID *int) string {
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	userPart := "anonymous"
	if userID != nil {
		userPart = strconv.Itoa(*userID)
	}

	ext := safeFileExtension(fileName)
	objectName := fmt.Sprintf("resource-%d-u%s%s", resourceNowFunc().UnixNano(), userPart, ext)
	if prefix == "" {
		return path.Join("resources", objectName)
	}
	return path.Join(prefix, "resources", objectName)
}

func (s *ResourceService) deleteObjectBestEffort(objectKey string) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" || strings.TrimSpace(s.BucketName) == "" {
		return
	}
	_ = deleteGCSObjectHook(s.BucketName, objectKey)
}

func (s *ResourceService) deleteStoredObjectBestEffort(objectKey string, fileURL string) {
	bucketName, resolvedObjectKey, err := s.resolveStoredObjectReference(objectKey, fileURL)
	if err != nil || strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		return
	}
	_ = deleteGCSObjectHook(bucketName, resolvedObjectKey)
}

func (s *ResourceService) resolveStoredObjectReference(objectKey string, fileURL string) (string, string, error) {
	objectKey = strings.TrimSpace(objectKey)
	fileURL = strings.TrimSpace(fileURL)
	if objectKey != "" && strings.TrimSpace(s.BucketName) != "" {
		return strings.TrimSpace(s.BucketName), objectKey, nil
	}
	if fileURL == "" {
		if objectKey != "" {
			return "", "", ErrMediaBucketNotConfigured
		}
		return "", "", fmt.Errorf("resource content is not available from storage")
	}
	if !looksLikeGCSReference(fileURL) {
		return "", "", fmt.Errorf("resource content is not available from storage")
	}

	bucketName, resolvedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL)
	if err != nil {
		if errors.Is(err, util.ErrBucketNameRequired) {
			return "", "", ErrMediaBucketNotConfigured
		}
		if errors.Is(err, util.ErrObjectNameRequired) {
			return "", "", fmt.Errorf("resource content is not available from storage")
		}
		return "", "", err
	}
	if strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		return "", "", fmt.Errorf("resource content is not available from storage")
	}
	return bucketName, resolvedObjectKey, nil
}

func (s *ResourceService) getResourceEntryModel(id int) (ResourceEntry, error) {
	var entry ResourceEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResourceEntry{}, ErrResourceNotFound
		}
		return ResourceEntry{}, err
	}
	return entry, nil
}

func resourceListItemFromModel(entry ResourceEntry) ResourceListItem {
	return ResourceListItem{
		ID:            entry.ID,
		Name:          entry.Name,
		Category:      entry.Category,
		CategoryLabel: resourceCategoryLabel(entry.Category),
		Visibility:    entry.Visibility,
		FileName:      entry.FileName,
		MimeType:      entry.MimeType,
		FileSize:      entry.FileSize,
		ContentURL:    buildResourceContentURL(entry.ID),
		CreatedAt:     entry.CreatedAt,
		UpdatedAt:     entry.UpdatedAt,
	}
}

func resourceMutationFromModel(entry ResourceEntry) *ResourceMutationResponse {
	return &ResourceMutationResponse{
		ID:         entry.ID,
		Name:       entry.Name,
		Category:   entry.Category,
		Visibility: entry.Visibility,
		UpdatedAt:  entry.UpdatedAt,
	}
}

func buildResourceContentURL(id int) string {
	return fmt.Sprintf("/api/resources/%d/content", id)
}

var resourceCategoryOrder = []string{
	ResourceCategoryBrandIdentity,
	ResourceCategoryGovernanceLegal,
	ResourceCategoryTrainingManuals,
	ResourceCategoryMediaKits,
}

func resourceCategoryLabel(category string) string {
	switch normalizeResourceCategory(category) {
	case ResourceCategoryBrandIdentity:
		return "Brand Identity"
	case ResourceCategoryGovernanceLegal:
		return "Governance & Legal"
	case ResourceCategoryTrainingManuals:
		return "Training & Manuals"
	case ResourceCategoryMediaKits:
		return "Media Kits"
	default:
		return "Resources"
	}
}

func normalizeResourceCategory(category string) string {
	return strings.ToLower(strings.TrimSpace(category))
}

func isAllowedResourceCategory(category string) bool {
	switch normalizeResourceCategory(category) {
	case ResourceCategoryBrandIdentity,
		ResourceCategoryGovernanceLegal,
		ResourceCategoryTrainingManuals,
		ResourceCategoryMediaKits:
		return true
	default:
		return false
	}
}

func isAllowedResourceVisibility(visibility string) bool {
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case ResourceVisibilityPublic, ResourceVisibilityInternal:
		return true
	default:
		return false
	}
}

func normalizeResourceFileType(fileType string) string {
	fileType = strings.ToLower(strings.TrimSpace(fileType))
	if fileType == "" {
		return "all"
	}
	return fileType
}

func isAllowedResourceFileType(fileType string) bool {
	switch normalizeResourceFileType(fileType) {
	case "all", "pdf", "document", "presentation", "spreadsheet", "image", "vector", "other":
		return true
	default:
		return false
	}
}

func resourceFileTypeCondition(fileType string) (string, []interface{}, error) {
	switch normalizeResourceFileType(fileType) {
	case "pdf":
		return "(LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?)", []interface{}{"%.pdf", "%pdf%"}, nil
	case "document":
		return "(LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?)", []interface{}{"%.doc", "%.docx", "%word%", "%document%"}, nil
	case "presentation":
		return "(LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?)", []interface{}{"%.ppt", "%.pptx", "%powerpoint%", "%presentation%"}, nil
	case "spreadsheet":
		return "(LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?)", []interface{}{"%.xls", "%.xlsx", "%excel%", "%sheet%"}, nil
	case "image":
		return "(LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ?)", []interface{}{"image/%", "%.png", "%.jpg", "%.jpeg", "%.webp"}, nil
	case "vector":
		return "(LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?)", []interface{}{"%.svg", "%svg%"}, nil
	case "other":
		return "NOT ((LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?) OR (LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?) OR (LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?) OR (LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?) OR (LOWER(COALESCE(mime_type, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(file_name, '')) LIKE ?) OR (LOWER(COALESCE(file_name, '')) LIKE ? OR LOWER(COALESCE(mime_type, '')) LIKE ?))",
			[]interface{}{
				"%.pdf", "%pdf%",
				"%.doc", "%.docx", "%word%", "%document%",
				"%.ppt", "%.pptx", "%powerpoint%", "%presentation%",
				"%.xls", "%.xlsx", "%excel%", "%sheet%",
				"image/%", "%.png", "%.jpg", "%.jpeg", "%.webp",
				"%.svg", "%svg%",
			}, nil
	default:
		return "", nil, fmt.Errorf("file_type is invalid")
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

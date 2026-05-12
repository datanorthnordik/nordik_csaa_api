package pages

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable         = errors.New("page store unavailable")
	ErrPageNotFound             = errors.New("page not found")
	ErrPageHeroImageNotFound    = errors.New("page hero image not found")
	ErrMediaBucketNotConfigured = errors.New("drive bucket is not configured")
)

var (
	pagesNowFunc          = time.Now
	uploadBase64ToGCSHook = func(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
		return util.UploadBase64ToGCS(base64Data, bucketName, objectName, contentType)
	}
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

var slugPattern = regexp.MustCompile(`^/[a-z0-9]+(?:/[a-z0-9]+|-[a-z0-9]+)*$|^/$`)

type PageService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

func (s *PageService) ListPages(filter PageListFilters) (*PageListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalized, err := normalizeListPagesFilter(filter)
	if err != nil {
		return nil, err
	}

	countQuery := s.DB.Model(&Page{})
	countQuery = applyPageListFilters(countQuery, normalized)

	var totalItems int64
	if err := countQuery.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	var items []PageListItem
	query := s.DB.Model(&Page{}).
		Select(`
			pages.id,
			pages.page_title,
			pages.url_slug,
			pages.parent_id,
			COALESCE(parent_pages.page_title, '') AS parent_page_title,
			COALESCE(parent_pages.url_slug, '') AS parent_page_url_slug,
			pages.status,
			pages.last_modified,
			pages.modified_by,
			pages.created_at,
			pages.updated_at,
			COALESCE(NULLIF(TRIM(modified_users.firstname || ' ' || modified_users.lastname), ''), modified_users.email, '') AS modified_by_name
		`).
		Joins(`LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by`).
		Joins(`LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id`)
	query = applyPageListFilters(query, normalized)

	query = query.Order(buildPageSortClause(normalized.SortBy, normalized.SortOrder))
	if normalized.UsePagination {
		offset := (normalized.Page - 1) * normalized.PageSize
		query = query.Offset(offset).Limit(normalized.PageSize)
	}

	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}
	if items == nil {
		items = make([]PageListItem, 0)
	}

	pagination := PageListPageMeta{
		TotalItems: totalItems,
	}
	if normalized.UsePagination {
		totalPages := 0
		if totalItems > 0 {
			totalPages = int((totalItems + int64(normalized.PageSize) - 1) / int64(normalized.PageSize))
		}
		pagination.Page = normalized.Page
		pagination.PageSize = normalized.PageSize
		pagination.TotalPages = totalPages
		pagination.HasNext = normalized.Page < totalPages
		pagination.HasPrev = normalized.Page > 1
	} else {
		pagination.Page = 1
		pagination.PageSize = len(items)
		if totalItems > 0 {
			pagination.TotalPages = 1
		}
	}

	return &PageListResponse{
		Items:      items,
		Pagination: pagination,
		Applied:    normalized,
	}, nil
}

func (s *PageService) GetPage(id int) (*PageDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var item PageDetailResponse
	if err := s.DB.Model(&Page{}).
		Select(`
			pages.id,
			pages.page_title,
			pages.url_slug,
			pages.parent_id,
			COALESCE(parent_pages.page_title, '') AS parent_page_title,
			COALESCE(parent_pages.url_slug, '') AS parent_page_url_slug,
			pages.status,
			pages.hero_image_enabled,
			pages.hero_image_url,
			pages.hero_image_object_key,
			pages.seo_page_title,
			pages.seo_page_description,
			pages.created_by,
			pages.modified_by,
			pages.last_modified,
			pages.created_at,
			pages.updated_at,
			COALESCE(NULLIF(TRIM(created_users.firstname || ' ' || created_users.lastname), ''), created_users.email, '') AS created_by_name,
			COALESCE(NULLIF(TRIM(modified_users.firstname || ' ' || modified_users.lastname), ''), modified_users.email, '') AS modified_by_name
		`).
		Joins(`LEFT JOIN users AS created_users ON created_users.id = pages.created_by`).
		Joins(`LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by`).
		Joins(`LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id`).
		Where("pages.id = ?", id).
		Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(item.HeroImageURL) != "" || strings.TrimSpace(item.HeroImageObjectKey) != "" {
		item.HeroImageFetchURL = buildPageHeroFetchURL(item.ID)
	}

	return &item, nil
}

func (s *PageService) GetPageHeroImageContent(id int) (*PageHeroImageContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var page Page
	if err := s.DB.Select("id", "hero_image_url", "hero_image_object_key", "hero_image_enabled").First(&page, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(page.HeroImageURL) == "" && strings.TrimSpace(page.HeroImageObjectKey) == "" {
		return nil, ErrPageHeroImageNotFound
	}

	bucketName, objectKey, err := s.resolveHeroImageObjectReference(page)
	if err != nil {
		return nil, err
	}

	content, contentType, err := downloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrPageHeroImageNotFound
		}
		return nil, err
	}

	return &PageHeroImageContent{
		Content:     content,
		ContentType: contentType,
		FileName:    buildPageHeroFileName(page, objectKey),
	}, nil
}

func (s *PageService) CreatePage(req SavePageRequest) (*PageMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalized, err := normalizeSavePageRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	uploadedObjects := make([]string, 0, 1)

	if err := s.validateParentPage(tx, 0, normalized); err != nil {
		tx.Rollback()
		return nil, err
	}

	page := buildPageModel(normalized)
	if page.ModifiedBy == nil {
		page.ModifiedBy = normalized.CreatedBy
	}

	if err := tx.Create(&page).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if normalized.HeroImageEnabled && normalized.HeroImage != nil {
		heroURL, heroObjectKey, uploadedObject, err := s.buildHeroImageFields(page.ID, *normalized.HeroImage)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}
		page.HeroImageURL = heroURL
		page.HeroImageObjectKey = heroObjectKey
		if err := tx.Model(&page).Updates(map[string]any{
			"hero_image_url":        heroURL,
			"hero_image_object_key": heroObjectKey,
		}).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &PageMutationResponse{
		ID:        page.ID,
		PageTitle: page.PageTitle,
		URLSlug:   page.URLSlug,
		ParentID:  page.ParentID,
		Status:    page.Status,
	}, nil
}

func (s *PageService) UpdatePage(id int, req SavePageRequest) (*PageMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalized, err := normalizeSavePageRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	uploadedObjects := make([]string, 0, 1)

	var page Page
	if err := tx.First(&page, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}

	if err := s.validateParentPage(tx, id, normalized); err != nil {
		tx.Rollback()
		return nil, err
	}

	oldHero := page
	applyPageRequest(&page, normalized)
	page.CreatedBy = oldHero.CreatedBy

	switch {
	case !normalized.HeroImageEnabled || normalized.RemoveHeroImage:
		page.HeroImageURL = ""
		page.HeroImageObjectKey = ""
	case normalized.HeroImage != nil:
		heroURL, heroObjectKey, uploadedObject, err := s.buildHeroImageFields(page.ID, *normalized.HeroImage)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}
		page.HeroImageURL = heroURL
		page.HeroImageObjectKey = heroObjectKey
	}

	if err := tx.Save(&page).Error; err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if shouldCleanupHeroImage(oldHero, page) {
		if err := s.cleanupSingleHeroObject(oldHero); err != nil {
			return nil, err
		}
	}

	return &PageMutationResponse{
		ID:        page.ID,
		PageTitle: page.PageTitle,
		URLSlug:   page.URLSlug,
		ParentID:  page.ParentID,
		Status:    page.Status,
	}, nil
}

func (s *PageService) DeletePage(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackOnPanic(tx)

	var page Page
	if err := tx.First(&page, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPageNotFound
		}
		return err
	}

	if err := tx.Delete(&page).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return s.cleanupSingleHeroObject(page)
}

func normalizeSavePageRequest(req SavePageRequest) (SavePageRequest, error) {
	req.PageTitle = strings.TrimSpace(req.PageTitle)
	req.URLSlug = normalizeURLSlug(req.URLSlug)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.SEOPageTitle = strings.TrimSpace(req.SEOPageTitle)
	req.SEOPageDescription = strings.TrimSpace(req.SEOPageDescription)

	if req.HeroImage != nil {
		cleaned := sanitizeUploadInput(*req.HeroImage)
		req.HeroImage = &cleaned
	}

	if req.Status == "" {
		req.Status = PageStatusDraft
	}

	if req.PageTitle == "" {
		return req, errors.New("page_title is required")
	}
	if req.URLSlug == "" {
		return req, errors.New("url_slug is required")
	}
	if !slugPattern.MatchString(req.URLSlug) {
		return req, errors.New("url_slug must be a valid slash-prefixed slug")
	}
	if req.ParentID != nil && *req.ParentID <= 0 {
		return req, errors.New("parent_id must be a positive integer")
	}
	if !isAllowed(req.Status, PageStatusDraft, PageStatusPublished) {
		return req, errors.New("invalid status")
	}

	if !req.HeroImageEnabled {
		req.RemoveHeroImage = true
		req.HeroImage = nil
	}

	return req, nil
}

func normalizeListPagesFilter(filter PageListFilters) (PageListFilters, error) {
	filter.SearchTerm = strings.TrimSpace(filter.SearchTerm)
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	filter.SortBy = strings.ToLower(strings.TrimSpace(filter.SortBy))
	filter.SortOrder = strings.ToLower(strings.TrimSpace(filter.SortOrder))

	if filter.UsePagination {
		if filter.Page <= 0 {
			filter.Page = 1
		}
		if filter.PageSize <= 0 {
			filter.PageSize = 10
		}
		if filter.PageSize > 100 {
			filter.PageSize = 100
		}
	}
	if filter.SortBy == "" {
		filter.SortBy = "last_modified"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}
	if filter.Status != "" && !isAllowed(filter.Status, PageStatusDraft, PageStatusPublished) {
		return filter, fmt.Errorf("invalid status %q", filter.Status)
	}
	if !isAllowed(filter.SortBy, "page_title", "url_slug", "status", "last_modified", "created_at", "updated_at") {
		return filter, errors.New("invalid sort_by")
	}
	if !isAllowed(filter.SortOrder, "asc", "desc") {
		return filter, errors.New("invalid sort_order")
	}

	return filter, nil
}

func applyPageListFilters(query *gorm.DB, filter PageListFilters) *gorm.DB {
	if filter.SearchTerm != "" {
		search := "%" + strings.ToLower(filter.SearchTerm) + "%"
		query = query.Where("(LOWER(page_title) LIKE ? OR LOWER(url_slug) LIKE ?)", search, search)
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	return query
}

func buildPageSortClause(sortBy string, sortOrder string) string {
	allowedColumns := map[string]string{
		"page_title":    "page_title",
		"url_slug":      "url_slug",
		"status":        "status",
		"last_modified": "last_modified",
		"created_at":    "created_at",
		"updated_at":    "updated_at",
	}
	column, ok := allowedColumns[sortBy]
	if !ok {
		column = "last_modified"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	return column + " " + strings.ToUpper(sortOrder)
}

func buildPageModel(req SavePageRequest) Page {
	return Page{
		PageTitle:          req.PageTitle,
		URLSlug:            req.URLSlug,
		ParentID:           req.ParentID,
		Status:             req.Status,
		HeroImageEnabled:   req.HeroImageEnabled,
		SEOPageTitle:       req.SEOPageTitle,
		SEOPageDescription: req.SEOPageDescription,
		CreatedBy:          req.CreatedBy,
		ModifiedBy:         req.ModifiedBy,
	}
}

func applyPageRequest(page *Page, req SavePageRequest) {
	page.PageTitle = req.PageTitle
	page.URLSlug = req.URLSlug
	page.ParentID = req.ParentID
	page.Status = req.Status
	page.HeroImageEnabled = req.HeroImageEnabled
	page.SEOPageTitle = req.SEOPageTitle
	page.SEOPageDescription = req.SEOPageDescription
	page.ModifiedBy = req.ModifiedBy
}

func sanitizeUploadInput(value PageUploadInput) PageUploadInput {
	value.FileName = strings.TrimSpace(value.FileName)
	value.MimeType = strings.TrimSpace(value.MimeType)
	value.DataBase64 = strings.TrimSpace(value.DataBase64)
	value.FileURL = strings.TrimSpace(value.FileURL)
	value.StorageURI = strings.TrimSpace(value.StorageURI)
	value.ObjectKey = strings.TrimSpace(value.ObjectKey)
	value.GCPObjectKey = strings.TrimSpace(value.GCPObjectKey)
	return value
}

func normalizeURLSlug(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Trim(value, "/")
	value = regexp.MustCompile(`/+`).ReplaceAllString(value, "/")
	if value == "" {
		return "/"
	}
	return "/" + value
}

func (s *PageService) validateParentPage(tx *gorm.DB, pageID int, req SavePageRequest) error {
	if req.ParentID == nil {
		return nil
	}

	parentID := *req.ParentID
	if parentID <= 0 {
		return errors.New("parent_id must be a positive integer")
	}
	if pageID > 0 && parentID == pageID {
		return errors.New("parent_id cannot reference the page itself")
	}

	var parent Page
	if err := tx.Model(&Page{}).
		Select("id", "url_slug").
		Take(&parent, parentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("parent_id references a page that does not exist")
		}
		return err
	}

	return validatePageSlugParentPrefix(req.URLSlug, parent.URLSlug)
}

func validatePageSlugParentPrefix(pageSlug string, parentSlug string) error {
	parentSlug = normalizeURLSlug(parentSlug)
	if parentSlug == "/" {
		if pageSlug == "/" {
			return fmt.Errorf("url_slug must be prefixed with parent page slug %q", parentSlug)
		}
		return nil
	}

	if !strings.HasPrefix(pageSlug, parentSlug+"/") {
		return fmt.Errorf("url_slug must be prefixed with parent page slug %q", parentSlug)
	}

	return nil
}

func (s *PageService) buildHeroImageFields(pageID int, input PageUploadInput) (string, string, string, error) {
	referenceURL := strings.TrimSpace(input.StorageURI)
	if referenceURL == "" {
		referenceURL = strings.TrimSpace(input.FileURL)
	}
	objectKey := strings.TrimSpace(input.ObjectKey)
	if objectKey == "" {
		objectKey = strings.TrimSpace(input.GCPObjectKey)
	}

	if len(input.Content) == 0 && strings.TrimSpace(input.DataBase64) == "" {
		if referenceURL == "" {
			return "", "", "", errors.New("hero_image is missing both uploaded file and file_url")
		}
		if objectKey == "" {
			_, parsedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), referenceURL)
			if err == nil {
				objectKey = s.relativeObjectKey(parsedObjectKey)
			}
		}
		return referenceURL, objectKey, "", nil
	}

	if strings.TrimSpace(s.BucketName) == "" {
		return "", "", "", ErrMediaBucketNotConfigured
	}

	objectName := s.heroImageObjectName(pageID, input.FileName, input.MimeType)
	storageObjectName := s.storageObjectName(objectName)
	var (
		fileURL string
		err     error
	)
	if len(input.Content) > 0 {
		fileURL, _, err = uploadBytesToGCSHook(
			input.Content,
			s.BucketName,
			storageObjectName,
			strings.TrimSpace(input.MimeType),
		)
	} else {
		fileURL, _, err = uploadBase64ToGCSHook(
			input.DataBase64,
			s.BucketName,
			storageObjectName,
			strings.TrimSpace(input.MimeType),
		)
	}
	if err != nil {
		return "", "", "", err
	}

	return fileURL, objectName, storageObjectName, nil
}

func (s *PageService) heroImageObjectName(pageID int, fileName string, mimeType string) string {
	timestamp := pagesNowFunc().UTC().Format("20060102150405")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = "hero"
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("pages/%d/hero_%s_%s%s", pageID, timestamp, base, ext)
}

func (s *PageService) cleanupSingleHeroObject(page Page) error {
	if strings.TrimSpace(page.HeroImageURL) == "" && strings.TrimSpace(page.HeroImageObjectKey) == "" {
		return nil
	}

	bucketName, objectKey, err := s.resolveHeroImageObjectReference(page)
	if err != nil {
		return err
	}

	return deleteGCSObjectHook(bucketName, objectKey)
}

func (s *PageService) cleanupObjects(objectNames []string) {
	for _, objectName := range objectNames {
		if strings.TrimSpace(objectName) == "" || strings.TrimSpace(s.BucketName) == "" {
			continue
		}
		_ = deleteGCSObjectHook(s.BucketName, objectName)
	}
}

func (s *PageService) resolveHeroImageObjectReference(page Page) (string, string, error) {
	objectKey := strings.TrimSpace(page.HeroImageObjectKey)
	if objectKey != "" {
		bucketName := strings.TrimSpace(s.BucketName)
		if bucketName != "" {
			return bucketName, s.storageObjectName(objectKey), nil
		}
		if strings.TrimSpace(page.HeroImageURL) == "" {
			return "", "", ErrMediaBucketNotConfigured
		}
	}

	bucketName, objectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), page.HeroImageURL)
	if err != nil {
		if errors.Is(err, util.ErrBucketNameRequired) {
			return "", "", ErrMediaBucketNotConfigured
		}
		return "", "", err
	}
	if strings.TrimSpace(bucketName) == "" {
		return "", "", ErrMediaBucketNotConfigured
	}
	return bucketName, objectKey, nil
}

func (s *PageService) storageObjectName(objectKey string) string {
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	if objectKey == "" {
		return prefix
	}
	if prefix == "" {
		return objectKey
	}
	if objectKey == prefix || strings.HasPrefix(objectKey, prefix+"/") {
		return objectKey
	}
	return path.Join(prefix, objectKey)
}

func (s *PageService) relativeObjectKey(objectKey string) string {
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	if prefix == "" {
		return objectKey
	}
	if objectKey == prefix {
		return ""
	}
	if strings.HasPrefix(objectKey, prefix+"/") {
		return strings.TrimPrefix(objectKey, prefix+"/")
	}
	return objectKey
}

func buildPageHeroFetchURL(pageID int) string {
	return fmt.Sprintf("/api/pages/%d/hero/content", pageID)
}

func buildPageHeroFileName(page Page, objectKey string) string {
	baseName := path.Base(strings.TrimSpace(objectKey))
	if baseName != "." && baseName != "/" && baseName != "" {
		return baseName
	}
	return "page-hero" + util.ExtFromFilenameOrMime("", "")
}

func shouldCleanupHeroImage(previous Page, next Page) bool {
	prevURL := strings.TrimSpace(previous.HeroImageURL)
	prevKey := strings.TrimSpace(previous.HeroImageObjectKey)
	nextURL := strings.TrimSpace(next.HeroImageURL)
	nextKey := strings.TrimSpace(next.HeroImageObjectKey)

	if prevURL == "" && prevKey == "" {
		return false
	}

	return prevURL != nextURL || prevKey != nextKey
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

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
	ErrPageCTAImageNotFound     = errors.New("page CTA image not found")
	ErrPageModuleManaged        = errors.New("module pages are managed elsewhere in the CMS and cannot be edited here")
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

type pageMenuItemParentSyncRow struct {
	ID       int
	MenuID   int
	ParentID *int
}

type pageMenuParentTargetRow struct {
	ID     int
	MenuID int
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
			pages.page_type,
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

	return s.lookupPageDetail(func(query *gorm.DB) *gorm.DB {
		return query.Where("pages.id = ?", id)
	})
}

func (s *PageService) GetPageBySlug(slug string) (*PageDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalizedSlug := normalizeURLSlug(slug)
	if normalizedSlug == "" {
		return nil, errors.New("slug is required")
	}

	return s.lookupPageDetail(func(query *gorm.DB) *gorm.DB {
		return query.Where("pages.url_slug = ? AND pages.status = ?", normalizedSlug, PageStatusPublished)
	})
}

func (s *PageService) lookupPageDetail(apply func(*gorm.DB) *gorm.DB) (*PageDetailResponse, error) {
	var item PageDetailResponse
	query := s.DB.Model(&Page{}).
		Select(`
			pages.id,
			pages.page_title,
			pages.url_slug,
			pages.parent_id,
			pages.page_type,
			COALESCE(parent_pages.page_title, '') AS parent_page_title,
			COALESCE(parent_pages.url_slug, '') AS parent_page_url_slug,
			pages.status,
			pages.hero_image_enabled,
			COALESCE(pages.hero_image_url, '') AS hero_image_url,
			COALESCE(pages.hero_image_object_key, '') AS hero_image_object_key,
			COALESCE(pages.seo_page_title, '') AS seo_page_title,
			COALESCE(pages.seo_page_description, '') AS seo_page_description,
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
		Joins(`LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id`)
	if apply != nil {
		query = apply(query)
	}

	if err := query.Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(item.HeroImageURL) != "" || strings.TrimSpace(item.HeroImageObjectKey) != "" {
		item.HeroImageFetchURL = buildPageHeroFetchURL(item.ID)
	}

	pageDetail, err := s.getPageContentDetail(item.ID)
	if err != nil {
		return nil, err
	}
	item.PageDetail = pageDetail

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

	uploadedObjects := make([]string, 0, 4)
	storedObjectsToCleanup := make([]pageStoredObject, 0, 4)

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

	detailUploadedObjects, detailCleanupObjects, err := s.savePageContentDetail(tx, page.ID, normalized.PageDetail, page.ModifiedBy)
	if err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		s.cleanupObjects(detailUploadedObjects)
		return nil, err
	}
	uploadedObjects = append(uploadedObjects, detailUploadedObjects...)
	storedObjectsToCleanup = append(storedObjectsToCleanup, detailCleanupObjects...)

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.cleanupStoredObjects(storedObjectsToCleanup); err != nil {
		return nil, err
	}

	return &PageMutationResponse{
		ID:        page.ID,
		PageTitle: page.PageTitle,
		URLSlug:   page.URLSlug,
		ParentID:  page.ParentID,
		PageType:  page.PageType,
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

	uploadedObjects := make([]string, 0, 4)
	storedObjectsToCleanup := make([]pageStoredObject, 0, 4)

	var page Page
	if err := tx.First(&page, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}
	if page.PageType == PageTypeModule {
		tx.Rollback()
		return nil, ErrPageModuleManaged
	}

	if err := s.validateParentPage(tx, id, normalized); err != nil {
		tx.Rollback()
		return nil, err
	}

	oldHero := page
	oldParentID := copyOptionalInt(page.ParentID)
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
	if !optionalIntsEqual(oldParentID, page.ParentID) {
		if err := s.syncMenuItemsForPageParent(tx, page.ID, page.ParentID); err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	detailUploadedObjects, detailCleanupObjects, err := s.savePageContentDetail(tx, page.ID, normalized.PageDetail, normalized.ModifiedBy)
	if err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		s.cleanupObjects(detailUploadedObjects)
		return nil, err
	}
	uploadedObjects = append(uploadedObjects, detailUploadedObjects...)
	storedObjectsToCleanup = append(storedObjectsToCleanup, detailCleanupObjects...)

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if shouldCleanupHeroImage(oldHero, page) {
		if err := s.cleanupSingleHeroObject(oldHero); err != nil {
			return nil, err
		}
	}
	if err := s.cleanupStoredObjects(storedObjectsToCleanup); err != nil {
		return nil, err
	}

	return &PageMutationResponse{
		ID:        page.ID,
		PageTitle: page.PageTitle,
		URLSlug:   page.URLSlug,
		ParentID:  page.ParentID,
		PageType:  page.PageType,
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

	storedObjectsToCleanup := make([]pageStoredObject, 0, 4)

	var page Page
	if err := tx.First(&page, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPageNotFound
		}
		return err
	}
	if page.PageType == PageTypeModule {
		tx.Rollback()
		return ErrPageModuleManaged
	}

	candidateDocuments, err := s.loadPageDocumentReferencesForPage(tx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Delete(&page).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, candidate := range candidateDocuments {
		cleanupObject, err := s.deleteOrphanPageDocument(tx, candidate.ID)
		if err != nil {
			tx.Rollback()
			return err
		}
		if cleanupObject != nil {
			storedObjectsToCleanup = append(storedObjectsToCleanup, *cleanupObject)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	if err := s.cleanupSingleHeroObject(page); err != nil {
		return err
	}

	return s.cleanupStoredObjects(storedObjectsToCleanup)
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
	if req.PageDetail != nil {
		normalizedDetail, err := normalizeSavePageDetailRequest(req.PageDetail)
		if err != nil {
			return req, err
		}
		req.PageDetail = normalizedDetail
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
		query = query.Where("(LOWER(pages.page_title) LIKE ? OR LOWER(pages.url_slug) LIKE ?)", search, search)
	}

	if filter.Status != "" {
		query = query.Where("pages.status = ?", filter.Status)
	}

	return query
}

func buildPageSortClause(sortBy string, sortOrder string) string {
	allowedColumns := map[string]string{
		"page_title":    "pages.page_title",
		"url_slug":      "pages.url_slug",
		"status":        "pages.status",
		"last_modified": "pages.last_modified",
		"created_at":    "pages.created_at",
		"updated_at":    "pages.updated_at",
	}
	column, ok := allowedColumns[sortBy]
	if !ok {
		column = "pages.last_modified"
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
		PageType:           PageTypePage,
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

func isEmptyPageUploadInput(value PageUploadInput) bool {
	return strings.TrimSpace(value.FileName) == "" &&
		strings.TrimSpace(value.MimeType) == "" &&
		strings.TrimSpace(value.DataBase64) == "" &&
		len(value.Content) == 0 &&
		strings.TrimSpace(value.FileURL) == "" &&
		strings.TrimSpace(value.StorageURI) == "" &&
		strings.TrimSpace(value.ObjectKey) == "" &&
		strings.TrimSpace(value.GCPObjectKey) == ""
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

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	return copyInt(*value)
}

func copyInt(value int) *int {
	copied := value
	return &copied
}

func optionalIntsEqual(left *int, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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

func (s *PageService) syncMenuItemsForPageParent(tx *gorm.DB, pageID int, parentPageID *int) error {
	items := make([]pageMenuItemParentSyncRow, 0)
	if err := tx.Table("menu_items").
		Select("id", "menu_id", "parent_id").
		Where("page_id = ?", pageID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	parentMenuItemIDByMenuID := make(map[int]int)
	if parentPageID != nil {
		parentItems := make([]pageMenuParentTargetRow, 0)
		if err := tx.Table("menu_items").
			Select("id", "menu_id").
			Where("page_id = ?", *parentPageID).
			Order("id ASC").
			Find(&parentItems).Error; err != nil {
			return err
		}
		for _, parentItem := range parentItems {
			parentMenuItemIDByMenuID[parentItem.MenuID] = parentItem.ID
		}
	}

	for _, item := range items {
		var nextParentID *int
		if parentMenuItemID, ok := parentMenuItemIDByMenuID[item.MenuID]; ok {
			nextParentID = copyInt(parentMenuItemID)
		}
		if optionalIntsEqual(item.ParentID, nextParentID) {
			continue
		}

		updateQuery := tx.Table("menu_items").Where("id = ?", item.ID)
		if nextParentID == nil {
			if err := updateQuery.Update("parent_id", gorm.Expr("NULL")).Error; err != nil {
				return err
			}
			continue
		}
		if err := updateQuery.Update("parent_id", *nextParentID).Error; err != nil {
			return err
		}
	}

	return nil
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

func (s *PageService) storePageSectionImageInput(sectionID int, input PageUploadInput, fieldName string) (string, string, string, error) {
	referenceURL := strings.TrimSpace(input.StorageURI)
	if referenceURL == "" {
		referenceURL = strings.TrimSpace(input.FileURL)
	}
	objectKey := strings.TrimSpace(input.ObjectKey)
	if objectKey == "" {
		objectKey = strings.TrimSpace(input.GCPObjectKey)
	}

	if len(input.Content) == 0 && strings.TrimSpace(input.DataBase64) == "" {
		if referenceURL == "" && objectKey == "" {
			return "", "", "", fmt.Errorf("%s is missing both uploaded file and file_url", fieldName)
		}
		if objectKey == "" && referenceURL != "" {
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

	objectName := s.pageSectionImageObjectName(sectionID, input.FileName, input.MimeType)
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

func (s *PageService) pageSectionImageObjectName(sectionID int, fileName string, mimeType string) string {
	timestamp := pagesNowFunc().UTC().Format("20060102150405")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = "cta-image"
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("pages/sections/%d/cta_image_%s_%s%s", sectionID, timestamp, base, ext)
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

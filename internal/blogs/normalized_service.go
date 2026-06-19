package blogs

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable               = errors.New("blog store unavailable")
	ErrBlogNotFound                   = errors.New("blog not found")
	ErrBlogCoverImageNotFound         = errors.New("blog cover image not found")
	ErrBlogSectionImageNotFound       = errors.New("blog section image not found")
	ErrBlogAnimationItemImageNotFound = errors.New("blog animation item image not found")
	ErrMediaBucketNotConfigured       = errors.New("drive bucket is not configured")
)

var (
	blogsNowFunc          = time.Now
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

type BlogService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

type blogStoredAsset struct {
	FileURL      string
	GCPObjectKey string
}

type blogStoredObject struct {
	ObjectKey  string
	StorageURL string
}

func (s *BlogService) ListBlogs(filter BlogListFilters) (*BlogListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	normalized, err := normalizeListBlogsFilter(filter)
	if err != nil {
		return nil, err
	}

	countQuery := applyBlogListFilters(s.DB.Model(&Blog{}), normalized)
	var totalItems int64
	if err := countQuery.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	var items []BlogListItem
	query := s.DB.Model(&Blog{}).
		Select(`
			blogs.id, blogs.publish_date, blogs.heading,
			COALESCE(blogs.description, '') AS description,
			COALESCE(blogs.cover_image_url, '') AS cover_image_url,
			COALESCE(blogs.cover_image_object_key, '') AS cover_image_object_key,
			blogs.updated_by, blogs.created_at, blogs.updated_at,
			COALESCE(NULLIF(TRIM(updated_users.firstname || ' ' || updated_users.lastname), ''), updated_users.email, '') AS updated_by_name
		`).
		Joins("LEFT JOIN users AS updated_users ON updated_users.id = blogs.updated_by")
	query = applyBlogListFilters(query, normalized).
		Order(buildBlogSortClause(normalized.SortBy, normalized.SortOrder))
	if normalized.UsePagination {
		query = query.Offset((normalized.Page - 1) * normalized.PageSize).Limit(normalized.PageSize)
	}
	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}
	if items == nil {
		items = make([]BlogListItem, 0)
	}
	for index := range items {
		if items[index].CoverImageURL != "" || items[index].CoverImageObjectKey != "" {
			items[index].CoverImageFetchURL = buildBlogCoverFetchURL(items[index].ID)
		}
	}

	pagination := BlogListPageMeta{Page: 1, PageSize: len(items), TotalItems: totalItems}
	if normalized.UsePagination {
		pagination.Page = normalized.Page
		pagination.PageSize = normalized.PageSize
		if totalItems > 0 {
			pagination.TotalPages = int((totalItems + int64(normalized.PageSize) - 1) / int64(normalized.PageSize))
		}
		pagination.HasNext = normalized.Page < pagination.TotalPages
		pagination.HasPrev = normalized.Page > 1
	} else if totalItems > 0 {
		pagination.TotalPages = 1
	}

	return &BlogListResponse{Items: items, Pagination: pagination, Applied: normalized}, nil
}

func (s *BlogService) GetBlog(id int) (*BlogDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	var response BlogDetailResponse
	err := s.DB.Model(&Blog{}).
		Select(`
			blogs.id, blogs.publish_date, blogs.heading,
			COALESCE(blogs.description, '') AS description,
			COALESCE(blogs.cover_image_url, '') AS cover_image_url,
			COALESCE(blogs.cover_image_object_key, '') AS cover_image_object_key,
			blogs.created_by, blogs.updated_by, blogs.created_at, blogs.updated_at,
			COALESCE(NULLIF(TRIM(created_users.firstname || ' ' || created_users.lastname), ''), created_users.email, '') AS created_by_name,
			COALESCE(NULLIF(TRIM(updated_users.firstname || ' ' || updated_users.lastname), ''), updated_users.email, '') AS updated_by_name
		`).
		Joins("LEFT JOIN users AS created_users ON created_users.id = blogs.created_by").
		Joins("LEFT JOIN users AS updated_users ON updated_users.id = blogs.updated_by").
		Where("blogs.id = ?", id).
		Take(&response).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBlogNotFound
		}
		return nil, err
	}
	if response.CoverImageURL != "" || response.CoverImageObjectKey != "" {
		response.CoverImageFetchURL = buildBlogCoverFetchURL(response.ID)
	}
	response.BlogDetail, err = s.getBlogContentDetail(id)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *BlogService) CreateBlog(req SaveBlogRequest) (*BlogMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	normalized, publishDate, err := normalizeSaveBlogRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBlogOnPanic(tx)
	uploadedObjects := make([]string, 0)

	blog := Blog{
		PublishDate: publishDate,
		Heading:     normalized.Heading,
		Description: normalized.Description,
		CreatedBy:   normalized.CreatedBy,
		UpdatedBy:   normalized.UpdatedBy,
	}
	if err := tx.Create(&blog).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if normalized.CoverImage != nil {
		asset, uploaded, err := s.storeBlogUploadInput(
			s.coverImageObjectName(blog.ID, normalized.CoverImage.FileName, normalized.CoverImage.MimeType),
			*normalized.CoverImage,
			"cover image",
		)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploaded != "" {
			uploadedObjects = append(uploadedObjects, uploaded)
		}
		blog.CoverImageURL = asset.FileURL
		blog.CoverImageObjectKey = asset.GCPObjectKey
		if err := tx.Model(&blog).Updates(map[string]any{
			"cover_image_url":        blog.CoverImageURL,
			"cover_image_object_key": blog.CoverImageObjectKey,
		}).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	sectionUploads, _, err := s.saveBlogContentDetail(tx, blog.ID, normalized.BlogDetail, normalized.CreatedBy)
	uploadedObjects = append(uploadedObjects, sectionUploads...)
	if err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}
	return &BlogMutationResponse{ID: blog.ID, PublishDate: blog.PublishDate, Heading: blog.Heading}, nil
}

func (s *BlogService) UpdateBlog(id int, req SaveBlogRequest) (*BlogMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	normalized, publishDate, err := normalizeSaveBlogRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBlogOnPanic(tx)

	var blog Blog
	if err := tx.First(&blog, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBlogNotFound
		}
		return nil, err
	}
	uploadedObjects := make([]string, 0)
	cleanupObjects := make([]blogStoredObject, 0)
	oldCover := blogStoredObject{ObjectKey: blog.CoverImageObjectKey, StorageURL: blog.CoverImageURL}

	blog.PublishDate = publishDate
	blog.Heading = normalized.Heading
	blog.Description = normalized.Description
	blog.UpdatedBy = normalized.UpdatedBy
	if normalized.RemoveCoverImage {
		blog.CoverImageURL = ""
		blog.CoverImageObjectKey = ""
	} else if normalized.CoverImage != nil {
		asset, uploaded, err := s.storeBlogUploadInput(
			s.coverImageObjectName(blog.ID, normalized.CoverImage.FileName, normalized.CoverImage.MimeType),
			*normalized.CoverImage,
			"cover image",
		)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if uploaded != "" {
			uploadedObjects = append(uploadedObjects, uploaded)
		}
		blog.CoverImageURL = asset.FileURL
		blog.CoverImageObjectKey = asset.GCPObjectKey
	}
	if err := tx.Save(&blog).Error; err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}
	newCover := blogStoredObject{ObjectKey: blog.CoverImageObjectKey, StorageURL: blog.CoverImageURL}
	if !sameBlogStoredObject(oldCover, newCover) {
		cleanupObjects = append(cleanupObjects, oldCover)
	}

	sectionUploads, sectionCleanup, err := s.saveBlogContentDetail(tx, blog.ID, normalized.BlogDetail, normalized.UpdatedBy)
	uploadedObjects = append(uploadedObjects, sectionUploads...)
	cleanupObjects = append(cleanupObjects, sectionCleanup...)
	if err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}
	if err := s.cleanupStoredObjects(cleanupObjects); err != nil {
		return nil, err
	}
	return &BlogMutationResponse{ID: blog.ID, PublishDate: blog.PublishDate, Heading: blog.Heading}, nil
}

func (s *BlogService) DeleteBlog(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}
	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackBlogOnPanic(tx)
	var blog Blog
	if err := tx.First(&blog, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrBlogNotFound
		}
		return err
	}
	objects, err := collectBlogStoredObjects(tx, blog)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&blog).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return s.cleanupStoredObjects(objects)
}

func (s *BlogService) GetBlogCoverImageContent(id int) (*BlogMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	var blog Blog
	if err := s.DB.Select("id", "cover_image_url", "cover_image_object_key").First(&blog, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBlogNotFound
		}
		return nil, err
	}
	if blog.CoverImageURL == "" && blog.CoverImageObjectKey == "" {
		return nil, ErrBlogCoverImageNotFound
	}
	return s.downloadBlogMedia(blogStoredObject{ObjectKey: blog.CoverImageObjectKey, StorageURL: blog.CoverImageURL}, ErrBlogCoverImageNotFound, "blog-cover")
}

func (s *BlogService) GetBlogSectionImageContent(id int, sectionID int) (*BlogMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	var module BlogSectionImageModule
	err := s.DB.Model(&BlogSectionImageModule{}).
		Joins("JOIN blog_sections ON blog_sections.id = blog_section_image_modules.blog_section_id").
		Joins("JOIN blog_details ON blog_details.id = blog_sections.blog_detail_id").
		Where("blog_details.blog_id = ? AND blog_sections.id = ?", id, sectionID).
		Take(&module).Error
	if err != nil {
		return nil, ErrBlogSectionImageNotFound
	}
	return s.downloadBlogMedia(blogStoredObject{ObjectKey: module.ImageObjectKey, StorageURL: module.ImageURL}, ErrBlogSectionImageNotFound, "blog-image")
}

func (s *BlogService) GetBlogAnimationItemImageContent(id int, sectionID int, itemID int) (*BlogMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	var item BlogAnimationItem
	err := s.DB.Model(&BlogAnimationItem{}).
		Joins("JOIN blog_sections ON blog_sections.id = blog_animation_items.blog_section_id").
		Joins("JOIN blog_details ON blog_details.id = blog_sections.blog_detail_id").
		Where("blog_details.blog_id = ? AND blog_sections.id = ? AND blog_animation_items.id = ?", id, sectionID, itemID).
		Take(&item).Error
	if err != nil {
		return nil, ErrBlogAnimationItemImageNotFound
	}
	return s.downloadBlogMedia(blogStoredObject{ObjectKey: item.ImageObjectKey, StorageURL: item.ImageURL}, ErrBlogAnimationItemImageNotFound, "animation-image")
}

func normalizeListBlogsFilter(filter BlogListFilters) (BlogListFilters, error) {
	filter.SearchTerm = strings.TrimSpace(filter.SearchTerm)
	filter.SortBy = strings.ToLower(strings.TrimSpace(filter.SortBy))
	filter.SortOrder = strings.ToLower(strings.TrimSpace(filter.SortOrder))
	if filter.SortBy == "" {
		filter.SortBy = "publish_date"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}
	if _, ok := map[string]bool{"publish_date": true, "heading": true, "updated_at": true, "created_at": true}[filter.SortBy]; !ok {
		return filter, errors.New("invalid sort_by")
	}
	if filter.SortOrder != "asc" && filter.SortOrder != "desc" {
		return filter, errors.New("invalid sort_order")
	}
	if filter.UsePagination {
		if filter.Page == 0 {
			filter.Page = 1
		}
		if filter.PageSize == 0 {
			filter.PageSize = 20
		}
		if filter.Page < 1 {
			return filter, errors.New("page must be at least 1")
		}
		if filter.PageSize < 1 || filter.PageSize > 100 {
			return filter, errors.New("page_size must be between 1 and 100")
		}
	}
	return filter, nil
}

func applyBlogListFilters(query *gorm.DB, filter BlogListFilters) *gorm.DB {
	if filter.SearchTerm != "" {
		search := "%" + strings.ToLower(filter.SearchTerm) + "%"
		query = query.Where("LOWER(blogs.heading) LIKE ? OR LOWER(blogs.description) LIKE ?", search, search)
	}
	return query
}

func buildBlogSortClause(sortBy, sortOrder string) string {
	columns := map[string]string{"publish_date": "blogs.publish_date", "heading": "blogs.heading", "updated_at": "blogs.updated_at", "created_at": "blogs.created_at"}
	column := columns[sortBy]
	if column == "" {
		column = "blogs.publish_date"
	}
	return column + " " + strings.ToUpper(sortOrder) + ", blogs.id " + strings.ToUpper(sortOrder)
}

func (s *BlogService) storeBlogUploadInput(objectKey string, input BlogUploadInput, fieldName string) (blogStoredAsset, string, error) {
	referenceURL := strings.TrimSpace(input.StorageURI)
	if referenceURL == "" {
		referenceURL = strings.TrimSpace(input.FileURL)
	}
	storedObjectKey := strings.TrimSpace(input.ObjectKey)
	if storedObjectKey == "" {
		storedObjectKey = strings.TrimSpace(input.GCPObjectKey)
	}
	if len(input.Content) == 0 && strings.TrimSpace(input.DataBase64) == "" {
		if referenceURL == "" && storedObjectKey == "" {
			return blogStoredAsset{}, "", fmt.Errorf("%s is missing both uploaded file and file_url", fieldName)
		}
		if storedObjectKey == "" && referenceURL != "" {
			_, parsedKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), referenceURL)
			if err == nil {
				storedObjectKey = s.relativeObjectKey(parsedKey)
			}
		}
		return blogStoredAsset{FileURL: referenceURL, GCPObjectKey: storedObjectKey}, "", nil
	}
	if strings.TrimSpace(s.BucketName) == "" {
		return blogStoredAsset{}, "", ErrMediaBucketNotConfigured
	}
	storageName := s.storageObjectName(objectKey)
	var fileURL string
	var err error
	if len(input.Content) > 0 {
		fileURL, _, err = uploadBytesToGCSHook(input.Content, s.BucketName, storageName, strings.TrimSpace(input.MimeType))
	} else {
		fileURL, _, err = uploadBase64ToGCSHook(input.DataBase64, s.BucketName, storageName, strings.TrimSpace(input.MimeType))
	}
	if err != nil {
		return blogStoredAsset{}, "", err
	}
	return blogStoredAsset{FileURL: fileURL, GCPObjectKey: objectKey}, storageName, nil
}

func (s *BlogService) coverImageObjectName(blogID int, fileName, mimeType string) string {
	return s.blogImageObjectName(fmt.Sprintf("blogs/%d", blogID), "cover", fileName, mimeType)
}

func (s *BlogService) sectionImageObjectName(blogID, sectionID int, fileName, mimeType string) string {
	return s.blogImageObjectName(fmt.Sprintf("blogs/%d/sections/%d", blogID, sectionID), "image", fileName, mimeType)
}

func (s *BlogService) animationItemImageObjectName(blogID, sectionID, itemID int, fileName, mimeType string) string {
	return s.blogImageObjectName(fmt.Sprintf("blogs/%d/sections/%d/items/%d", blogID, sectionID, itemID), "image", fileName, mimeType)
}

func (s *BlogService) blogImageObjectName(directory, fallback, fileName, mimeType string) string {
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = fallback
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("%s/%s_%s_%s%s", directory, fallback, blogsNowFunc().UTC().Format("20060102150405"), base, ext)
}

func (s *BlogService) storageObjectName(objectKey string) string {
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	if prefix == "" || objectKey == prefix || strings.HasPrefix(objectKey, prefix+"/") {
		return objectKey
	}
	return path.Join(prefix, objectKey)
}

func (s *BlogService) relativeObjectKey(objectKey string) string {
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	if prefix != "" && strings.HasPrefix(objectKey, prefix+"/") {
		return strings.TrimPrefix(objectKey, prefix+"/")
	}
	return objectKey
}

func (s *BlogService) downloadBlogMedia(object blogStoredObject, notFound error, fallback string) (*BlogMediaContent, error) {
	if object.ObjectKey == "" && object.StorageURL == "" {
		return nil, notFound
	}
	content, contentType, err := s.downloadStoredObject(object)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, notFound
		}
		return nil, err
	}
	return &BlogMediaContent{Content: content, ContentType: contentType, FileName: buildStoredFileName(object.ObjectKey, object.StorageURL, contentType, fallback)}, nil
}

func (s *BlogService) downloadStoredObject(item blogStoredObject) ([]byte, string, error) {
	bucket, objectName, err := s.resolveStoredObjectReference(item)
	if err != nil {
		return nil, "", err
	}
	return downloadGCSObjectHook(bucket, objectName)
}

func (s *BlogService) resolveStoredObjectReference(item blogStoredObject) (string, string, error) {
	bucket := strings.TrimSpace(s.BucketName)
	objectName := s.storageObjectName(item.ObjectKey)
	if objectName != "" && bucket != "" {
		return bucket, objectName, nil
	}
	parsedBucket, parsedObject, err := util.ParseGCSObjectReference(bucket, item.StorageURL)
	if err != nil {
		return "", "", err
	}
	return parsedBucket, parsedObject, nil
}

func (s *BlogService) cleanupObjects(objectNames []string) {
	for _, objectName := range objectNames {
		if strings.TrimSpace(objectName) != "" && strings.TrimSpace(s.BucketName) != "" {
			_ = deleteGCSObjectHook(s.BucketName, objectName)
		}
	}
}

func (s *BlogService) cleanupStoredObjects(items []blogStoredObject) error {
	seen := make(map[string]struct{})
	for _, item := range items {
		fingerprint := blogStoredObjectFingerprint(item)
		if fingerprint == "" {
			continue
		}
		if _, exists := seen[fingerprint]; exists {
			continue
		}
		seen[fingerprint] = struct{}{}
		bucket, objectName, err := s.resolveStoredObjectReference(item)
		if err != nil {
			continue
		}
		if err := deleteGCSObjectHook(bucket, objectName); err != nil && !errors.Is(err, util.ErrObjectNotFound) {
			return err
		}
	}
	return nil
}

func sameBlogStoredObject(left, right blogStoredObject) bool {
	return blogStoredObjectFingerprint(left) == blogStoredObjectFingerprint(right)
}

func blogStoredObjectFingerprint(item blogStoredObject) string {
	if key := strings.TrimSpace(item.ObjectKey); key != "" {
		return "key:" + key
	}
	if value := strings.TrimSpace(item.StorageURL); value != "" {
		return "url:" + value
	}
	return ""
}

func buildBlogCoverFetchURL(blogID int) string {
	return fmt.Sprintf("/api/blogs/%d/cover/content", blogID)
}

func buildBlogSectionImageFetchURL(blogID, sectionID int) string {
	return fmt.Sprintf("/api/blogs/%d/sections/%d/image/content", blogID, sectionID)
}

func buildBlogAnimationItemImageFetchURL(blogID, sectionID, itemID int) string {
	return fmt.Sprintf("/api/blogs/%d/sections/%d/items/%d/image/content", blogID, sectionID, itemID)
}

func buildStoredFileName(objectKey, storageURL, contentType, fallback string) string {
	name := path.Base(strings.TrimSpace(objectKey))
	if name == "" || name == "." {
		if parsed, err := url.Parse(strings.TrimSpace(storageURL)); err == nil {
			name = path.Base(parsed.Path)
		}
	}
	if name == "" || name == "." {
		name = fallback + util.ExtFromFilenameOrMime("", contentType)
	}
	return name
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func boolPtr(value bool) *bool { return &value }

func rollbackBlogOnPanic(tx *gorm.DB) {
	if recovered := recover(); recovered != nil {
		tx.Rollback()
		panic(recovered)
	}
}

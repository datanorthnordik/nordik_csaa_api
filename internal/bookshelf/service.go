package bookshelf

import (
	"errors"
	"fmt"
	"net/url"
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
	ErrStoreUnavailable         = errors.New("bookshelf service unavailable")
	ErrBookNotFound             = errors.New("book not found")
	ErrBookContentNotFound      = errors.New("book content not found")
	ErrAuthorImageNotFound      = errors.New("author image not found")
	ErrCoverImageNotFound       = errors.New("cover image not found")
	ErrMediaBucketNotConfigured = errors.New("media bucket is not configured")
)

var (
	bookshelfNowFunc     = time.Now
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

type BookshelfService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

func (s *BookshelfService) ListBooks(filter ListBookshelfFilter) (*BookshelfListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalizedFilter, err := normalizeListBookshelfFilter(filter)
	if err != nil {
		return nil, err
	}

	query := s.applyListFilters(s.DB.Model(&BookshelfEntry{}), normalizedFilter)

	var totalItems int64
	if err := query.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	var withCoverCount int64
	if err := query.Session(&gorm.Session{}).
		Where("COALESCE(NULLIF(BTRIM(cover_image_file_url), ''), NULLIF(BTRIM(cover_image_file_name), '')) IS NOT NULL").
		Count(&withCoverCount).Error; err != nil {
		return nil, err
	}

	var rows []BookshelfEntry
	if err := query.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "updated_at"}, Desc: true}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true}).
		Offset((normalizedFilter.Page - 1) * normalizedFilter.PageSize).
		Limit(normalizedFilter.PageSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]BookshelfListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, bookshelfListItemFromModel(row))
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(normalizedFilter.PageSize) - 1) / int64(normalizedFilter.PageSize))
	}

	return &BookshelfListResponse{
		Items: items,
		Pagination: BookshelfListPageMeta{
			Page:       normalizedFilter.Page,
			PageSize:   normalizedFilter.PageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    normalizedFilter.Page < totalPages,
			HasPrev:    normalizedFilter.Page > 1 && totalPages > 0,
		},
		Summary: BookshelfListSummary{
			WithCoverCount:    withCoverCount,
			WithoutCoverCount: totalItems - withCoverCount,
		},
		Applied: BookshelfListAppliedFilters{
			Page:       normalizedFilter.Page,
			PageSize:   normalizedFilter.PageSize,
			SearchTerm: normalizedFilter.SearchTerm,
		},
	}, nil
}

func (s *BookshelfService) GetBook(id int) (*BookshelfDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getBookshelfEntryModel(id)
	if err != nil {
		return nil, err
	}

	resp := BookshelfDetailResponse(bookshelfListItemFromModel(entry))
	return &resp, nil
}

func (s *BookshelfService) GetBookContent(id int) (*BookshelfContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getBookshelfEntryModel(id)
	if err != nil {
		return nil, err
	}
	if !bookshelfHasBookUpload(entry) {
		return nil, fmt.Errorf("book does not have downloadable content")
	}

	bucketName, objectKey, err := s.resolveStoredObjectReference(entry.BookGCPObjectKey, entry.BookFileURL, "book content is not available from storage")
	if err != nil {
		return nil, err
	}

	data, contentType, err := downloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrBookContentNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = entry.BookMimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return &BookshelfContent{
		Content:     data,
		ContentType: contentType,
		FileName:    entry.BookFileName,
	}, nil
}

func (s *BookshelfService) GetCoverImageContent(id int) (*BookshelfContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getBookshelfEntryModel(id)
	if err != nil {
		return nil, err
	}
	if !bookshelfHasCoverImage(entry) {
		return nil, fmt.Errorf("book does not have a cover image")
	}

	bucketName, objectKey, err := s.resolveStoredObjectReference(entry.CoverImageGCPObjectKey, entry.CoverImageFileURL, "cover image is not available from storage")
	if err != nil {
		return nil, err
	}

	data, contentType, err := downloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrCoverImageNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = entry.CoverImageMimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return &BookshelfContent{
		Content:     data,
		ContentType: contentType,
		FileName:    entry.CoverImageFileName,
	}, nil
}

func (s *BookshelfService) GetAuthorImageContent(id int) (*BookshelfContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getBookshelfEntryModel(id)
	if err != nil {
		return nil, err
	}
	if !bookshelfHasAuthorImage(entry) {
		return nil, fmt.Errorf("book does not have an author image")
	}

	bucketName, objectKey, err := s.resolveStoredObjectReference(entry.AuthorImageGCPObjectKey, entry.AuthorImageFileURL, "author image is not available from storage")
	if err != nil {
		return nil, err
	}

	data, contentType, err := downloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrAuthorImageNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = entry.AuthorImageMimeType
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return &BookshelfContent{
		Content:     data,
		ContentType: contentType,
		FileName:    entry.AuthorImageFileName,
	}, nil
}

func (s *BookshelfService) CreateBook(req SaveBookshelfEntryRequest, userID *int) (*BookshelfMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, err := normalizeSaveBookshelfEntryRequest(req)
	if err != nil {
		return nil, err
	}

	bookFileURL, bookObjectKey, bookFileName, bookMimeType, bookFileSize, uploadedBookKey, err := s.resolveRequiredUpload(cleanReq.BookUpload, userID, "book", "book")
	if err != nil {
		return nil, err
	}

	authorImageFileURL, authorImageObjectKey, authorImageFileName, authorImageMimeType, authorImageFileSize, uploadedAuthorImageKey, err := s.resolveOptionalUpload(cleanReq.AuthorImage, userID, "author-image", "author image")
	if err != nil {
		s.deleteObjectBestEffort(uploadedBookKey)
		return nil, err
	}

	coverFileURL, coverObjectKey, coverFileName, coverMimeType, coverFileSize, uploadedCoverKey, err := s.resolveOptionalUpload(cleanReq.CoverImage, userID, "cover", "cover image")
	if err != nil {
		s.deleteObjectBestEffort(uploadedBookKey)
		s.deleteObjectBestEffort(uploadedAuthorImageKey)
		return nil, err
	}

	entry := BookshelfEntry{
		Author:                  cleanReq.Author,
		Title:                   cleanReq.Title,
		BookLink:                cleanReq.BookLink,
		AuthorBio:               cleanReq.AuthorBio,
		BookTeaser:              cleanReq.BookTeaser,
		Description:             cleanReq.Description,
		BookFileName:            bookFileName,
		BookGCPObjectKey:        bookObjectKey,
		BookFileURL:             bookFileURL,
		BookMimeType:            bookMimeType,
		BookFileSize:            bookFileSize,
		AuthorImageFileName:     authorImageFileName,
		AuthorImageGCPObjectKey: authorImageObjectKey,
		AuthorImageFileURL:      authorImageFileURL,
		AuthorImageMimeType:     authorImageMimeType,
		AuthorImageFileSize:     authorImageFileSize,
		CoverImageFileName:      coverFileName,
		CoverImageGCPObjectKey:  coverObjectKey,
		CoverImageFileURL:       coverFileURL,
		CoverImageMimeType:      coverMimeType,
		CoverImageFileSize:      coverFileSize,
		CreatedBy:               userID,
		UpdatedBy:               userID,
	}

	if err := s.DB.Create(&entry).Error; err != nil {
		s.deleteObjectBestEffort(uploadedBookKey)
		s.deleteObjectBestEffort(uploadedAuthorImageKey)
		s.deleteObjectBestEffort(uploadedCoverKey)
		return nil, err
	}

	return bookshelfMutationFromModel(entry), nil
}

func (s *BookshelfService) UpdateBook(id int, req SaveBookshelfEntryRequest, userID *int) (*BookshelfMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	entry, err := s.getBookshelfEntryModel(id)
	if err != nil {
		return nil, err
	}

	cleanReq, err := normalizeSaveBookshelfEntryRequest(req)
	if err != nil {
		return nil, err
	}

	oldBookObjectKey := entry.BookGCPObjectKey
	oldBookFileURL := entry.BookFileURL
	oldAuthorImageObjectKey := entry.AuthorImageGCPObjectKey
	oldAuthorImageFileURL := entry.AuthorImageFileURL
	oldCoverObjectKey := entry.CoverImageGCPObjectKey
	oldCoverFileURL := entry.CoverImageFileURL
	newUploadedBookKey := ""
	newUploadedAuthorImageKey := ""
	newUploadedCoverKey := ""
	shouldDeleteOldBook := false
	shouldDeleteOldAuthorImage := false
	shouldDeleteOldCover := false

	entry.Author = cleanReq.Author
	entry.Title = cleanReq.Title
	entry.BookLink = cleanReq.BookLink
	entry.AuthorBio = cleanReq.AuthorBio
	entry.BookTeaser = cleanReq.BookTeaser
	entry.Description = cleanReq.Description
	entry.UpdatedBy = userID

	if hasBookshelfUploadInput(cleanReq.BookUpload) {
		fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, resolveErr := s.resolveRequiredUpload(cleanReq.BookUpload, userID, "book", "book")
		if resolveErr != nil {
			return nil, resolveErr
		}
		shouldDeleteOldBook = bookshelfHasBookUpload(entry)
		entry.BookFileURL = fileURL
		entry.BookGCPObjectKey = objectKey
		entry.BookFileName = fileName
		entry.BookMimeType = mimeType
		entry.BookFileSize = fileSize
		newUploadedBookKey = uploadedKey
	} else if !bookshelfHasBookUpload(entry) {
		return nil, fmt.Errorf("book upload is required")
	}

	switch {
	case hasBookshelfUploadInput(cleanReq.AuthorImage):
		fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, resolveErr := s.resolveOptionalUpload(cleanReq.AuthorImage, userID, "author-image", "author image")
		if resolveErr != nil {
			s.deleteObjectBestEffort(newUploadedBookKey)
			return nil, resolveErr
		}
		shouldDeleteOldAuthorImage = bookshelfHasAuthorImage(entry)
		entry.AuthorImageFileURL = fileURL
		entry.AuthorImageGCPObjectKey = objectKey
		entry.AuthorImageFileName = fileName
		entry.AuthorImageMimeType = mimeType
		entry.AuthorImageFileSize = fileSize
		newUploadedAuthorImageKey = uploadedKey
	case cleanReq.RemoveAuthorImage:
		shouldDeleteOldAuthorImage = bookshelfHasAuthorImage(entry)
		clearBookshelfAuthorImageFields(&entry)
	}

	switch {
	case hasBookshelfUploadInput(cleanReq.CoverImage):
		fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, resolveErr := s.resolveOptionalUpload(cleanReq.CoverImage, userID, "cover", "cover image")
		if resolveErr != nil {
			s.deleteObjectBestEffort(newUploadedBookKey)
			s.deleteObjectBestEffort(newUploadedAuthorImageKey)
			return nil, resolveErr
		}
		shouldDeleteOldCover = bookshelfHasCoverImage(entry)
		entry.CoverImageFileURL = fileURL
		entry.CoverImageGCPObjectKey = objectKey
		entry.CoverImageFileName = fileName
		entry.CoverImageMimeType = mimeType
		entry.CoverImageFileSize = fileSize
		newUploadedCoverKey = uploadedKey
	case cleanReq.RemoveCoverImage:
		shouldDeleteOldCover = bookshelfHasCoverImage(entry)
		clearBookshelfCoverFields(&entry)
	}

	if err := s.DB.Save(&entry).Error; err != nil {
		s.deleteObjectBestEffort(newUploadedBookKey)
		s.deleteObjectBestEffort(newUploadedAuthorImageKey)
		s.deleteObjectBestEffort(newUploadedCoverKey)
		return nil, err
	}

	if shouldDeleteOldBook && (newUploadedBookKey == "" || oldBookObjectKey != entry.BookGCPObjectKey || oldBookFileURL != entry.BookFileURL) {
		s.deleteStoredObjectBestEffort(oldBookObjectKey, oldBookFileURL)
	}
	if shouldDeleteOldAuthorImage && (newUploadedAuthorImageKey == "" || oldAuthorImageObjectKey != entry.AuthorImageGCPObjectKey || oldAuthorImageFileURL != entry.AuthorImageFileURL) {
		s.deleteStoredObjectBestEffort(oldAuthorImageObjectKey, oldAuthorImageFileURL)
	}
	if shouldDeleteOldCover && (newUploadedCoverKey == "" || oldCoverObjectKey != entry.CoverImageGCPObjectKey || oldCoverFileURL != entry.CoverImageFileURL) {
		s.deleteStoredObjectBestEffort(oldCoverObjectKey, oldCoverFileURL)
	}

	return bookshelfMutationFromModel(entry), nil
}

func (s *BookshelfService) DeleteBook(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	entry, err := s.getBookshelfEntryModel(id)
	if err != nil {
		return err
	}

	if err := s.DB.Delete(&entry).Error; err != nil {
		return err
	}

	if bookshelfHasBookUpload(entry) {
		s.deleteStoredObjectBestEffort(entry.BookGCPObjectKey, entry.BookFileURL)
	}
	if bookshelfHasAuthorImage(entry) {
		s.deleteStoredObjectBestEffort(entry.AuthorImageGCPObjectKey, entry.AuthorImageFileURL)
	}
	if bookshelfHasCoverImage(entry) {
		s.deleteStoredObjectBestEffort(entry.CoverImageGCPObjectKey, entry.CoverImageFileURL)
	}
	return nil
}

func normalizeListBookshelfFilter(filter ListBookshelfFilter) (ListBookshelfFilter, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 10
	}
	filter.SearchTerm = strings.TrimSpace(filter.SearchTerm)
	return filter, nil
}

func normalizeSaveBookshelfEntryRequest(req SaveBookshelfEntryRequest) (SaveBookshelfEntryRequest, error) {
	req.Author = strings.TrimSpace(req.Author)
	req.Title = strings.TrimSpace(req.Title)
	req.BookLink = strings.TrimSpace(req.BookLink)
	req.AuthorBio = strings.TrimSpace(req.AuthorBio)
	req.BookTeaser = strings.TrimSpace(req.BookTeaser)
	req.Description = strings.TrimSpace(req.Description)

	if req.Author == "" {
		return req, fmt.Errorf("author is required")
	}
	if len(req.Author) > 255 {
		return req, fmt.Errorf("author must be 255 characters or fewer")
	}
	if req.Title == "" {
		return req, fmt.Errorf("title is required")
	}
	if len(req.Title) > 255 {
		return req, fmt.Errorf("title must be 255 characters or fewer")
	}
	if req.BookLink != "" {
		normalizedURL, err := normalizeBookshelfExternalURL(req.BookLink)
		if err != nil {
			return req, err
		}
		req.BookLink = normalizedURL
	}
	if req.Description == "" {
		return req, fmt.Errorf("description is required")
	}
	return req, nil
}

func normalizeBookshelfExternalURL(value string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("book_link must be a valid absolute URL")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("book_link must be a valid absolute URL")
	}
}

func (s *BookshelfService) resolveRequiredUpload(input *BookshelfUploadInput, userID *int, kind string, displayName string) (fileURL string, objectKey string, fileName string, mimeType string, fileSize int64, uploadedKey string, err error) {
	if input == nil {
		return "", "", "", "", 0, "", fmt.Errorf("%s upload is required", displayName)
	}

	fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err = s.resolveOptionalUpload(input, userID, kind, displayName)
	if err != nil {
		return "", "", "", "", 0, uploadedKey, err
	}
	if fileURL == "" {
		return "", "", "", "", 0, uploadedKey, fmt.Errorf("%s upload is required", displayName)
	}
	return fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, nil
}

func (s *BookshelfService) resolveOptionalUpload(input *BookshelfUploadInput, userID *int, kind string, displayName string) (fileURL string, objectKey string, fileName string, mimeType string, fileSize int64, uploadedKey string, err error) {
	if !hasBookshelfUploadInput(input) {
		return "", "", "", "", 0, "", nil
	}
	if input == nil {
		return "", "", "", "", 0, "", nil
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

		objectKey = s.buildObjectKey(kind, fileName, userID)
		uploadedURL, uploadedSize, uploadErr := uploadBytesToGCSHook(input.Content, s.BucketName, objectKey, mimeType)
		if uploadErr != nil {
			return "", "", "", "", 0, "", uploadErr
		}
		if strings.TrimSpace(uploadedURL) == "" {
			return "", objectKey, "", "", 0, objectKey, fmt.Errorf("%s upload returned an empty file url", displayName)
		}
		fileURL = uploadedURL
		fileSize = uploadedSize
		uploadedKey = objectKey
	}

	if fileURL == "" {
		return "", uploadedKey, "", "", 0, uploadedKey, nil
	}
	if fileName == "" {
		fileName = storedFilename(objectKey, kind+"-file")
	}
	if objectKey == "" && looksLikeGCSReference(fileURL) {
		if _, resolvedObjectKey, parseErr := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL); parseErr == nil {
			objectKey = resolvedObjectKey
		}
	}

	return fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, nil
}

func (s *BookshelfService) applyListFilters(query *gorm.DB, filter ListBookshelfFilter) *gorm.DB {
	if searchTerm := strings.TrimSpace(filter.SearchTerm); searchTerm != "" {
		pattern := "%" + strings.ToLower(searchTerm) + "%"
		query = query.Where(
			"LOWER(COALESCE(author, '')) LIKE ? OR LOWER(COALESCE(title, '')) LIKE ? OR LOWER(COALESCE(author_bio, '')) LIKE ? OR LOWER(COALESCE(book_teaser, '')) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ? OR LOWER(COALESCE(book_link, '')) LIKE ? OR LOWER(COALESCE(book_file_name, '')) LIKE ? OR LOWER(COALESCE(author_image_file_name, '')) LIKE ? OR LOWER(COALESCE(cover_image_file_name, '')) LIKE ?",
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}
	return query
}

func (s *BookshelfService) buildObjectKey(kind string, fileName string, userID *int) string {
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	userPart := "anonymous"
	if userID != nil {
		userPart = strconv.Itoa(*userID)
	}

	ext := safeFileExtension(fileName)
	objectName := fmt.Sprintf("%s-%d-u%s%s", kind, bookshelfNowFunc().UnixNano(), userPart, ext)
	basePath := path.Join("bookshelf", kind+"s", objectName)
	if prefix == "" {
		return basePath
	}
	return path.Join(prefix, basePath)
}

func (s *BookshelfService) deleteObjectBestEffort(objectKey string) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" || strings.TrimSpace(s.BucketName) == "" {
		return
	}
	_ = deleteGCSObjectHook(s.BucketName, objectKey)
}

func (s *BookshelfService) deleteStoredObjectBestEffort(objectKey string, fileURL string) {
	bucketName, resolvedObjectKey, err := s.resolveStoredObjectReference(objectKey, fileURL, "")
	if err != nil || strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		return
	}
	_ = deleteGCSObjectHook(bucketName, resolvedObjectKey)
}

func (s *BookshelfService) resolveStoredObjectReference(objectKey string, fileURL string, emptyMessage string) (string, string, error) {
	objectKey = strings.TrimSpace(objectKey)
	fileURL = strings.TrimSpace(fileURL)
	if objectKey != "" && strings.TrimSpace(s.BucketName) != "" {
		return strings.TrimSpace(s.BucketName), objectKey, nil
	}
	if fileURL == "" {
		if objectKey != "" {
			return "", "", ErrMediaBucketNotConfigured
		}
		if emptyMessage == "" {
			return "", "", fmt.Errorf("stored content is not available from storage")
		}
		return "", "", fmt.Errorf("%s", emptyMessage)
	}
	if !looksLikeGCSReference(fileURL) {
		if emptyMessage == "" {
			return "", "", fmt.Errorf("stored content is not available from storage")
		}
		return "", "", fmt.Errorf("%s", emptyMessage)
	}

	bucketName, resolvedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL)
	if err != nil {
		if errors.Is(err, util.ErrBucketNameRequired) {
			return "", "", ErrMediaBucketNotConfigured
		}
		if errors.Is(err, util.ErrObjectNameRequired) {
			if emptyMessage == "" {
				return "", "", fmt.Errorf("stored content is not available from storage")
			}
			return "", "", fmt.Errorf("%s", emptyMessage)
		}
		return "", "", err
	}
	if strings.TrimSpace(bucketName) == "" || strings.TrimSpace(resolvedObjectKey) == "" {
		if emptyMessage == "" {
			return "", "", fmt.Errorf("stored content is not available from storage")
		}
		return "", "", fmt.Errorf("%s", emptyMessage)
	}
	return bucketName, resolvedObjectKey, nil
}

func (s *BookshelfService) getBookshelfEntryModel(id int) (BookshelfEntry, error) {
	var entry BookshelfEntry
	if err := s.DB.First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BookshelfEntry{}, ErrBookNotFound
		}
		return BookshelfEntry{}, err
	}
	return entry, nil
}

func bookshelfListItemFromModel(entry BookshelfEntry) BookshelfListItem {
	item := BookshelfListItem{
		ID:                  entry.ID,
		Author:              entry.Author,
		Title:               entry.Title,
		BookLink:            entry.BookLink,
		AuthorBio:           entry.AuthorBio,
		BookTeaser:          entry.BookTeaser,
		Description:         entry.Description,
		BookFileName:        entry.BookFileName,
		BookMimeType:        entry.BookMimeType,
		BookFileSize:        entry.BookFileSize,
		AuthorImageFileName: entry.AuthorImageFileName,
		AuthorImageMimeType: entry.AuthorImageMimeType,
		AuthorImageFileSize: entry.AuthorImageFileSize,
		HasAuthorImage:      bookshelfHasAuthorImage(entry),
		CoverImageFileName:  entry.CoverImageFileName,
		CoverImageMimeType:  entry.CoverImageMimeType,
		CoverImageFileSize:  entry.CoverImageFileSize,
		HasCoverImage:       bookshelfHasCoverImage(entry),
		CreatedAt:           entry.CreatedAt,
		UpdatedAt:           entry.UpdatedAt,
	}
	if bookshelfHasBookUpload(entry) {
		item.BookContentURL = buildBookshelfBookContentURL(entry.ID)
	}
	if item.HasAuthorImage {
		item.AuthorImageContentURL = buildBookshelfAuthorImageContentURL(entry.ID)
	}
	if item.HasCoverImage {
		item.CoverImageContentURL = buildBookshelfCoverContentURL(entry.ID)
	}
	return item
}

func bookshelfMutationFromModel(entry BookshelfEntry) *BookshelfMutationResponse {
	return &BookshelfMutationResponse{
		ID:        entry.ID,
		Author:    entry.Author,
		Title:     entry.Title,
		UpdatedAt: entry.UpdatedAt,
	}
}

func buildBookshelfBookContentURL(id int) string {
	return fmt.Sprintf("/api/bookshelf/%d/book/content", id)
}

func buildBookshelfAuthorImageContentURL(id int) string {
	return fmt.Sprintf("/api/bookshelf/%d/author-image/content", id)
}

func buildBookshelfCoverContentURL(id int) string {
	return fmt.Sprintf("/api/bookshelf/%d/cover/content", id)
}

func bookshelfHasBookUpload(entry BookshelfEntry) bool {
	return strings.TrimSpace(entry.BookFileName) != "" || strings.TrimSpace(entry.BookFileURL) != ""
}

func bookshelfHasAuthorImage(entry BookshelfEntry) bool {
	return strings.TrimSpace(entry.AuthorImageFileName) != "" || strings.TrimSpace(entry.AuthorImageFileURL) != ""
}

func bookshelfHasCoverImage(entry BookshelfEntry) bool {
	return strings.TrimSpace(entry.CoverImageFileName) != "" || strings.TrimSpace(entry.CoverImageFileURL) != ""
}

func hasBookshelfUploadInput(input *BookshelfUploadInput) bool {
	if input == nil {
		return false
	}
	return len(input.Content) > 0 ||
		strings.TrimSpace(input.FileURL) != "" ||
		strings.TrimSpace(input.GCPObjectKey) != "" ||
		strings.TrimSpace(input.FileName) != ""
}

func clearBookshelfCoverFields(entry *BookshelfEntry) {
	if entry == nil {
		return
	}
	entry.CoverImageFileName = ""
	entry.CoverImageGCPObjectKey = ""
	entry.CoverImageFileURL = ""
	entry.CoverImageMimeType = ""
	entry.CoverImageFileSize = 0
}

func clearBookshelfAuthorImageFields(entry *BookshelfEntry) {
	if entry == nil {
		return
	}
	entry.AuthorImageFileName = ""
	entry.AuthorImageGCPObjectKey = ""
	entry.AuthorImageFileURL = ""
	entry.AuthorImageMimeType = ""
	entry.AuthorImageFileSize = 0
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

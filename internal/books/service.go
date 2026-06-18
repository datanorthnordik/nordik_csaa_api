package books

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"path"
	"sort"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable            = errors.New("book service unavailable")
	ErrBookNotFound                = errors.New("book not found")
	ErrBookVersionNotFound         = errors.New("book version not found")
	ErrBookSubmissionNotFound      = errors.New("book submission not found")
	ErrBookPDFNotFound             = errors.New("book pdf not found")
	ErrBookSubmissionImageNotFound = errors.New("book submission image not found")
	ErrBookActiveVersionNotFound   = errors.New("active book version not found")
	ErrMediaBucketNotConfigured    = errors.New("book media bucket is not configured")
)

var (
	booksNowFunc             = time.Now
	bookUploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
		return util.UploadBytesToGCS(data, bucketName, objectName, contentType)
	}
	bookDownloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return util.ReadGCSObject(bucketName, objectName)
	}
	bookDeleteGCSObjectHook = func(bucketName, objectName string) error {
		return util.DeleteGCSObject(bucketName, objectName)
	}
)

type BookEmailSender interface {
	SendEmail(to []string, subject string, body string) error
}

type BookService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
	EmailSender  BookEmailSender
}

type storedBookObjectRef struct {
	ObjectKey string
	FileURL   string
}

type storedBookUpload struct {
	FileName    string
	FileURL     string
	StorageURI  string
	ObjectKey   string
	MimeType    string
	FileSize    int64
	UploadedKey string
}

type bookVersionSubmissionCountRow struct {
	BookVersionID int    `gorm:"column:book_version_id"`
	Status        string `gorm:"column:status"`
	Count         int64  `gorm:"column:count"`
}

type bookSectionCountRow struct {
	BookVersionID int   `gorm:"column:book_version_id"`
	Count         int64 `gorm:"column:count"`
}

type bookFieldCountRow struct {
	BookVersionID int   `gorm:"column:book_version_id"`
	Count         int64 `gorm:"column:count"`
}

type bookSubmissionSectionCountRow struct {
	TargetSectionID int   `gorm:"column:target_section_id"`
	Count           int64 `gorm:"column:count"`
}

func (s *BookService) ListBooks() ([]BookSummaryResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var books []Book
	if err := s.DB.Order("updated_at DESC").Order("id DESC").Find(&books).Error; err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return []BookSummaryResponse{}, nil
	}

	bookIDs := make([]int, 0, len(books))
	activeVersionIDs := make([]int, 0, len(books))
	for _, book := range books {
		bookIDs = append(bookIDs, book.ID)
		if book.ActiveVersionID != nil && *book.ActiveVersionID > 0 {
			activeVersionIDs = append(activeVersionIDs, *book.ActiveVersionID)
		}
	}

	versionCounts, err := s.versionCountByBook(bookIDs)
	if err != nil {
		return nil, err
	}
	pendingCounts, err := s.pendingSubmissionCountByBook(bookIDs)
	if err != nil {
		return nil, err
	}
	activeVersionNumbers, err := s.versionNumberByID(activeVersionIDs)
	if err != nil {
		return nil, err
	}

	resp := make([]BookSummaryResponse, 0, len(books))
	for _, book := range books {
		item := BookSummaryResponse{
			ID:                     book.ID,
			Title:                  strings.TrimSpace(book.Title),
			Description:            strings.TrimSpace(book.Description),
			ActiveVersionID:        cloneIntPointer(book.ActiveVersionID),
			VersionCount:           versionCounts[book.ID],
			PendingSubmissionCount: pendingCounts[book.ID],
			CreatedAt:              book.CreatedAt,
			UpdatedAt:              book.UpdatedAt,
		}
		if book.ActiveVersionID != nil {
			if versionNumber, ok := activeVersionNumbers[*book.ActiveVersionID]; ok {
				item.ActiveVersionNumber = cloneIntPointer(&versionNumber)
			}
		}
		resp = append(resp, item)
	}

	return resp, nil
}

func (s *BookService) GetBook(bookID int) (*BookDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	book, err := s.getBookModel(bookID)
	if err != nil {
		return nil, err
	}

	var versions []BookVersion
	if err := s.DB.Where("book_id = ?", bookID).Order("version_number DESC").Order("id DESC").Find(&versions).Error; err != nil {
		return nil, err
	}

	versionIDs := make([]int, 0, len(versions))
	for _, version := range versions {
		versionIDs = append(versionIDs, version.ID)
	}

	sectionCounts, err := s.sectionCountByVersion(versionIDs)
	if err != nil {
		return nil, err
	}
	fieldCounts, err := s.fieldCountByVersion(versionIDs)
	if err != nil {
		return nil, err
	}
	submissionCounts, err := s.submissionCountByVersion(versionIDs)
	if err != nil {
		return nil, err
	}

	response := &BookDetailResponse{
		ID:                      book.ID,
		Title:                   strings.TrimSpace(book.Title),
		Description:             strings.TrimSpace(book.Description),
		AdminNotificationEmails: cloneStringSlice([]string(book.AdminNotificationEmails)),
		ActiveVersionID:         cloneIntPointer(book.ActiveVersionID),
		Versions:                make([]BookVersionSummaryResponse, 0, len(versions)),
		CreatedAt:               book.CreatedAt,
		UpdatedAt:               book.UpdatedAt,
	}

	for _, version := range versions {
		counts := submissionCounts[version.ID]
		response.Versions = append(response.Versions, BookVersionSummaryResponse{
			ID:                      version.ID,
			VersionNumber:           version.VersionNumber,
			IsActive:                book.ActiveVersionID != nil && *book.ActiveVersionID == version.ID,
			SourcePageCount:         version.SourcePageCount,
			SectionsCount:           sectionCounts[version.ID],
			FieldsCount:             fieldCounts[version.ID],
			ApprovedSubmissionCount: counts.Approved,
			PendingSubmissionCount:  counts.Pending,
			CreatedAt:               version.CreatedAt,
			UpdatedAt:               version.UpdatedAt,
			LastGeneratedAt:         cloneTimePointer(version.LastGeneratedAt),
		})
	}

	return response, nil
}

func (s *BookService) CreateBook(req SaveBookRequest) (*BookMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeSaveBookRequest(req)
	if err != nil {
		return nil, err
	}

	book := Book{
		Title:                   req.Title,
		Description:             req.Description,
		AdminNotificationEmails: pq.StringArray(req.AdminNotificationEmails),
		CreatedBy:               cloneIntPointer(req.CreatedBy),
		UpdatedBy:               cloneIntPointer(req.UpdatedBy),
	}

	if err := s.DB.Create(&book).Error; err != nil {
		return nil, err
	}

	return &BookMutationResponse{
		ID:          book.ID,
		Title:       book.Title,
		Description: book.Description,
		UpdatedAt:   book.UpdatedAt,
	}, nil
}

func (s *BookService) UpdateBook(bookID int, req SaveBookRequest) (*BookMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeSaveBookRequest(req)
	if err != nil {
		return nil, err
	}

	book, err := s.getBookModel(bookID)
	if err != nil {
		return nil, err
	}

	book.Title = req.Title
	book.Description = req.Description
	book.AdminNotificationEmails = pq.StringArray(req.AdminNotificationEmails)
	book.UpdatedBy = cloneIntPointer(req.UpdatedBy)

	if err := s.DB.Save(book).Error; err != nil {
		return nil, err
	}

	return &BookMutationResponse{
		ID:          book.ID,
		Title:       book.Title,
		Description: book.Description,
		UpdatedAt:   book.UpdatedAt,
	}, nil
}

func (s *BookService) CreateBookVersion(bookID int, req SaveBookVersionRequest) (*BookVersionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeSaveBookVersionRequest(req, true)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBooksOnPanic(tx)

	book, err := s.getBookModelTx(tx, bookID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	versionNumber, err := s.nextVersionNumber(tx, bookID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := validateManualInitialVersionNumber(versionNumber); err != nil {
		tx.Rollback()
		return nil, err
	}

	uploadedObjects := make([]string, 0, 2)
	version := BookVersion{
		BookID:                    bookID,
		VersionNumber:             versionNumber,
		SourcePageCount:           req.SourcePageCount,
		ContentTemplatePageNumber: req.ContentTemplatePageNumber,
		SectionTemplatePageNumber: req.SectionTemplatePageNumber,
		AllowPageImage:            req.AllowPageImage,
		AllowNewSections:          req.AllowNewSections,
		LayoutSettings:            cloneRawJSON(req.LayoutSettings),
		CreatedBy:                 cloneIntPointer(req.CreatedBy),
		UpdatedBy:                 cloneIntPointer(req.UpdatedBy),
	}

	sourceUpload, err := s.storeVersionPDF(bookID, versionNumber, "source", *req.SourcePDF)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if sourceUpload.UploadedKey != "" {
		uploadedObjects = append(uploadedObjects, sourceUpload.UploadedKey)
	}
	version.SourcePDFFileName = sourceUpload.FileName
	version.SourcePDFFileURL = sourceUpload.FileURL
	version.SourcePDFStorageURI = sourceUpload.StorageURI
	version.SourcePDFObjectKey = sourceUpload.ObjectKey

	sourcePDFContent, err := s.resolveBookUploadContent(req.SourcePDF, sourceUpload, ErrBookPDFNotFound)
	if err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	version.LayoutSettings = deriveInitialBookLayoutSettings(sourcePDFContent, req.ContentTemplatePageNumber, req.SectionTemplatePageNumber)

	if err := tx.Create(&version).Error; err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}

	if err := s.syncVersionSections(tx, version.ID, version.SourcePageCount, req.Sections); err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	if err := s.syncVersionFields(tx, version.ID, req.Fields); err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	if err := s.recomputeVersionSectionPageBounds(tx, version); err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}

	sections, err := s.listVersionSectionModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	fields, err := s.listVersionFieldModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	generatedUpload, err := s.generateAndStoreVersionPDF(bookID, &version, sourcePDFContent, sections, fields, nil, nil, nil)
	if err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	if generatedUpload.UploadedKey != "" {
		uploadedObjects = append(uploadedObjects, generatedUpload.UploadedKey)
	}
	applyStoredGeneratedPDFUpload(&version, generatedUpload, booksNowFunc())
	if err := tx.Model(&BookVersion{}).Where("id = ?", version.ID).Updates(map[string]any{
		"layout_settings":           cloneRawJSON(version.LayoutSettings),
		"generated_pdf_file_name":   nullableString(version.GeneratedPDFFileName),
		"generated_pdf_file_url":    nullableString(version.GeneratedPDFFileURL),
		"generated_pdf_storage_uri": nullableString(version.GeneratedPDFStorageURI),
		"generated_pdf_object_key":  nullableString(version.GeneratedPDFObjectKey),
		"last_generated_at":         version.LastGeneratedAt,
	}).Error; err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Model(&Book{}).Where("id = ?", book.ID).Updates(map[string]any{
		"active_version_id": version.ID,
		"updated_by":        nullableInt(req.UpdatedBy),
	}).Error; err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	book.ActiveVersionID = cloneInt(version.ID)

	if err := tx.Commit().Error; err != nil {
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}

	return &BookVersionMutationResponse{
		ID:            version.ID,
		BookID:        version.BookID,
		VersionNumber: version.VersionNumber,
		IsActive:      book.ActiveVersionID != nil && *book.ActiveVersionID == version.ID,
		UpdatedAt:     version.UpdatedAt,
	}, nil
}

func (s *BookService) UpdateBookVersion(bookID int, versionID int, req SaveBookVersionRequest) (*BookVersionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	return nil, errors.New("book version setup can only be created once and cannot be edited later")
}

func (s *BookService) SetActiveVersion(bookID int, versionID int, userID *int) (*BookVersionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBooksOnPanic(tx)

	book, err := s.getBookModelTx(tx, bookID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	version, err := s.getBookVersionModelTx(tx, bookID, versionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	book.ActiveVersionID = cloneInt(version.ID)
	book.UpdatedBy = cloneIntPointer(userID)

	if err := tx.Model(&Book{}).Where("id = ?", book.ID).Updates(map[string]any{
		"active_version_id": version.ID,
		"updated_by":        nullableInt(userID),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &BookVersionMutationResponse{
		ID:            version.ID,
		BookID:        version.BookID,
		VersionNumber: version.VersionNumber,
		IsActive:      true,
		UpdatedAt:     version.UpdatedAt,
	}, nil
}

func (s *BookService) GetBookVersionDetail(bookID int, versionID int) (*BookVersionDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	book, err := s.getBookModel(bookID)
	if err != nil {
		return nil, err
	}
	version, err := s.getBookVersionModel(bookID, versionID)
	if err != nil {
		return nil, err
	}

	sections, err := s.listVersionSectionResponses(version.ID)
	if err != nil {
		return nil, err
	}
	fields, err := s.listVersionFieldResponses(version.ID)
	if err != nil {
		return nil, err
	}
	approvedSubmissions, err := s.listSubmissionResponses(bookID, ListBookSubmissionsFilter{
		VersionID: version.ID,
		Status:    BookSubmissionStatusApproved,
	}, "reviewed_at ASC NULLS LAST, id ASC")
	if err != nil {
		return nil, err
	}

	resp := &BookVersionDetailResponse{
		ID:                        version.ID,
		BookID:                    version.BookID,
		VersionNumber:             version.VersionNumber,
		IsActive:                  book.ActiveVersionID != nil && *book.ActiveVersionID == version.ID,
		SourcePageCount:           version.SourcePageCount,
		ContentTemplatePageNumber: version.ContentTemplatePageNumber,
		SectionTemplatePageNumber: version.SectionTemplatePageNumber,
		AllowPageImage:            version.AllowPageImage,
		AllowNewSections:          version.AllowNewSections,
		LayoutSettings:            cloneRawJSON(version.LayoutSettings),
		SourcePDFFetchURL:         buildSourcePDFFetchURL(book.ID, version.ID),
		GeneratedPDFFetchURL:      "",
		Sections:                  sections,
		Fields:                    fields,
		ApprovedSubmissions:       approvedSubmissions,
		LastGeneratedAt:           cloneTimePointer(version.LastGeneratedAt),
		CreatedAt:                 version.CreatedAt,
		UpdatedAt:                 version.UpdatedAt,
	}
	if hasStoredBookFile(version.GeneratedPDFObjectKey, version.GeneratedPDFStorageURI, version.GeneratedPDFFileURL) {
		resp.GeneratedPDFFetchURL = buildGeneratedPDFFetchURL(book.ID, version.ID)
	}
	if resp.ApprovedSubmissions == nil {
		resp.ApprovedSubmissions = []BookSubmissionResponse{}
	}

	return resp, nil
}

func (s *BookService) UploadGeneratedPDF(bookID int, versionID int, input BookUploadInput, userID *int) (*BookVersionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	input = sanitizeBookUploadInput(input)
	if isEmptyBookUploadInput(input) {
		return nil, errors.New("generated_pdf is required")
	}
	if err := validatePDFUploadInput(input, "generated_pdf"); err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBooksOnPanic(tx)

	book, err := s.getBookModelTx(tx, bookID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	version, err := s.getBookVersionModelTx(tx, bookID, versionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	oldGenerated := storedBookObjectRef{
		ObjectKey: version.GeneratedPDFObjectKey,
		FileURL:   coalesceString(version.GeneratedPDFStorageURI, version.GeneratedPDFFileURL),
	}
	storedUpload, err := s.storeVersionPDF(bookID, version.VersionNumber, "generated", input)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	version.GeneratedPDFFileName = storedUpload.FileName
	version.GeneratedPDFFileURL = storedUpload.FileURL
	version.GeneratedPDFStorageURI = storedUpload.StorageURI
	version.GeneratedPDFObjectKey = storedUpload.ObjectKey
	version.UpdatedBy = cloneIntPointer(userID)
	now := booksNowFunc()
	version.LastGeneratedAt = &now

	if err := tx.Save(version).Error; err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects([]string{storedUpload.UploadedKey})
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupUploadedBookObjects([]string{storedUpload.UploadedKey})
		return nil, err
	}

	if shouldCleanupStoredBookObject(oldGenerated, storedBookObjectRef{ObjectKey: version.GeneratedPDFObjectKey, FileURL: version.GeneratedPDFStorageURI}) {
		s.cleanupStoredBookObjectsBestEffort([]storedBookObjectRef{oldGenerated})
	}

	return &BookVersionMutationResponse{
		ID:            version.ID,
		BookID:        book.ID,
		VersionNumber: version.VersionNumber,
		IsActive:      book.ActiveVersionID != nil && *book.ActiveVersionID == version.ID,
		UpdatedAt:     version.UpdatedAt,
	}, nil
}

func (s *BookService) GetSourcePDFContent(bookID int, versionID int) (*BookPDFContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	version, err := s.getBookVersionModel(bookID, versionID)
	if err != nil {
		return nil, err
	}

	return s.downloadVersionPDF(version.SourcePDFObjectKey, coalesceString(version.SourcePDFStorageURI, version.SourcePDFFileURL), version.SourcePDFFileName)
}

func (s *BookService) GetGeneratedPDFContent(bookID int, versionID int) (*BookPDFContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	version, err := s.getBookVersionModel(bookID, versionID)
	if err != nil {
		return nil, err
	}
	if !hasStoredBookFile(version.GeneratedPDFObjectKey, version.GeneratedPDFStorageURI, version.GeneratedPDFFileURL) {
		return nil, ErrBookPDFNotFound
	}

	return s.downloadVersionPDF(version.GeneratedPDFObjectKey, coalesceString(version.GeneratedPDFStorageURI, version.GeneratedPDFFileURL), version.GeneratedPDFFileName)
}

func (s *BookService) GetSubmissionImageContent(bookID int, submissionID int) (*SubmissionImageContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	submission, err := s.getSubmissionModel(bookID, submissionID)
	if err != nil {
		return nil, err
	}
	if !hasStoredBookFile(submission.ImageObjectKey, submission.ImageStorageURI, submission.ImageFileURL) {
		return nil, ErrBookSubmissionImageNotFound
	}

	data, contentType, err := s.downloadStoredBookObject(storedBookObjectRef{
		ObjectKey: submission.ImageObjectKey,
		FileURL:   coalesceString(submission.ImageStorageURI, submission.ImageFileURL),
	}, ErrBookSubmissionImageNotFound)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = submission.ImageMimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	fileName := strings.TrimSpace(submission.ImageFileName)
	if fileName == "" {
		fileName = buildStoredBookFileName(submission.ImageObjectKey, "submission-image")
	}

	return &SubmissionImageContent{
		Content:     data,
		ContentType: contentType,
		FileName:    fileName,
	}, nil
}

func (s *BookService) ListBookSubmissions(bookID int, filter ListBookSubmissionsFilter) ([]BookSubmissionResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}
	if _, err := s.getBookModel(bookID); err != nil {
		return nil, err
	}

	if filter.VersionID < 0 {
		return nil, errors.New("version_id must be a positive integer")
	}
	filter.Status = strings.TrimSpace(strings.ToLower(filter.Status))
	if filter.Status != "" && !isAllowedBookValue(filter.Status, BookSubmissionStatusPending, BookSubmissionStatusApproved, BookSubmissionStatusRejected) {
		return nil, errors.New("invalid status")
	}

	return s.listSubmissionResponses(bookID, filter, "created_at DESC, id DESC")
}

func (s *BookService) GetBookSubmission(bookID int, submissionID int) (*BookSubmissionResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	submission, err := s.getSubmissionModel(bookID, submissionID)
	if err != nil {
		return nil, err
	}

	responses, err := s.buildSubmissionResponses([]BookSubmission{*submission})
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, ErrBookSubmissionNotFound
	}
	return &responses[0], nil
}

func (s *BookService) CreatePublicSubmission(bookID int, req SaveBookSubmissionRequest) (*BookSubmissionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBooksOnPanic(tx)

	book, err := s.getBookModelTx(tx, bookID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if book.ActiveVersionID == nil || *book.ActiveVersionID <= 0 {
		tx.Rollback()
		return nil, ErrBookActiveVersionNotFound
	}
	version, err := s.getBookVersionModelTx(tx, bookID, *book.ActiveVersionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	fields, err := s.listVersionFieldModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	sections, err := s.listVersionSectionModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	req, submitterEmail, values, err := normalizeBookSubmissionRequest(req, *version, fields, sections)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	submission := BookSubmission{
		BookID:          book.ID,
		BookVersionID:   version.ID,
		TargetSectionID: cloneIntPointer(req.TargetSectionID),
		NewSectionName:  nullableStringPointer(req.NewSectionName),
		Status:          BookSubmissionStatusPending,
		SubmitterEmail:  submitterEmail,
	}

	if err := tx.Create(&submission).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if req.Image != nil && !isEmptyBookUploadInput(*req.Image) {
		imageUpload, storeErr := s.storeSubmissionImage(book.ID, submission.ID, *req.Image)
		if storeErr != nil {
			tx.Rollback()
			return nil, storeErr
		}
		submission.ImageFileName = imageUpload.FileName
		submission.ImageFileURL = imageUpload.FileURL
		submission.ImageStorageURI = imageUpload.StorageURI
		submission.ImageObjectKey = imageUpload.ObjectKey
		submission.ImageMimeType = imageUpload.MimeType
		submission.ImageFileSize = imageUpload.FileSize

		if err := tx.Model(&BookSubmission{}).Where("id = ?", submission.ID).Updates(map[string]any{
			"image_file_name":   nullableString(submission.ImageFileName),
			"image_file_url":    nullableString(submission.ImageFileURL),
			"image_storage_uri": nullableString(submission.ImageStorageURI),
			"image_object_key":  nullableString(submission.ImageObjectKey),
			"image_mime_type":   nullableString(submission.ImageMimeType),
			"image_file_size":   nullableInt64Value(submission.ImageFileSize),
		}).Error; err != nil {
			tx.Rollback()
			s.cleanupUploadedBookObjects([]string{imageUpload.UploadedKey})
			return nil, err
		}
	}

	for _, value := range values {
		value.BookSubmissionID = submission.ID
		if err := tx.Create(&value).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.sendAdminNewSubmissionEmail(book, version, submission, sections)

	return &BookSubmissionMutationResponse{
		ID:        submission.ID,
		Status:    submission.Status,
		UpdatedAt: submission.UpdatedAt,
	}, nil
}

func (s *BookService) UpdateBookSubmission(bookID int, submissionID int, req UpdateBookSubmissionRequest) (*BookSubmissionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBooksOnPanic(tx)

	submission, err := s.getSubmissionModelTx(tx, bookID, submissionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if submission.Status == BookSubmissionStatusApproved {
		tx.Rollback()
		return nil, errors.New("approved submissions cannot be edited")
	}

	version, err := s.getBookVersionModelTx(tx, bookID, submission.BookVersionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	fields, err := s.listVersionFieldModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	sections, err := s.listVersionSectionModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if req.RemoveImage && req.Image != nil && !isEmptyBookUploadInput(*req.Image) {
		tx.Rollback()
		return nil, errors.New("image cannot be uploaded when remove_image is true")
	}

	createLikeReq := SaveBookSubmissionRequest{
		TargetSectionID: req.TargetSectionID,
		NewSectionName:  req.NewSectionName,
		FieldValues:     req.FieldValues,
		Image:           req.Image,
	}
	createLikeReq, submitterEmail, values, err := normalizeBookSubmissionRequest(createLikeReq, *version, fields, sections)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	oldImage := storedBookObjectRef{
		ObjectKey: submission.ImageObjectKey,
		FileURL:   coalesceString(submission.ImageStorageURI, submission.ImageFileURL),
	}
	cleanupObjects := make([]storedBookObjectRef, 0, 1)

	submission.TargetSectionID = cloneIntPointer(createLikeReq.TargetSectionID)
	submission.NewSectionName = nullableStringPointer(createLikeReq.NewSectionName)
	submission.SubmitterEmail = submitterEmail

	switch {
	case req.RemoveImage:
		submission.ImageFileName = ""
		submission.ImageFileURL = ""
		submission.ImageStorageURI = ""
		submission.ImageObjectKey = ""
		submission.ImageMimeType = ""
		submission.ImageFileSize = 0
		if shouldCleanupStoredBookObject(oldImage, storedBookObjectRef{}) {
			cleanupObjects = append(cleanupObjects, oldImage)
		}
	case createLikeReq.Image != nil && !isEmptyBookUploadInput(*createLikeReq.Image):
		imageUpload, storeErr := s.storeSubmissionImage(bookID, submission.ID, *createLikeReq.Image)
		if storeErr != nil {
			tx.Rollback()
			return nil, storeErr
		}
		submission.ImageFileName = imageUpload.FileName
		submission.ImageFileURL = imageUpload.FileURL
		submission.ImageStorageURI = imageUpload.StorageURI
		submission.ImageObjectKey = imageUpload.ObjectKey
		submission.ImageMimeType = imageUpload.MimeType
		submission.ImageFileSize = imageUpload.FileSize
		if shouldCleanupStoredBookObject(oldImage, storedBookObjectRef{ObjectKey: submission.ImageObjectKey, FileURL: submission.ImageStorageURI}) {
			cleanupObjects = append(cleanupObjects, oldImage)
		}
	}

	if err := tx.Save(submission).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Where("book_submission_id = ?", submission.ID).Delete(&BookSubmissionValue{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, value := range values {
		value.BookSubmissionID = submission.ID
		if err := tx.Create(&value).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.cleanupStoredBookObjectsBestEffort(cleanupObjects)

	return &BookSubmissionMutationResponse{
		ID:        submission.ID,
		Status:    submission.Status,
		UpdatedAt: submission.UpdatedAt,
	}, nil
}

func (s *BookService) ApproveBookSubmission(bookID int, submissionID int, userID *int) (*BookSubmissionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBooksOnPanic(tx)

	submission, err := s.getSubmissionModelTx(tx, bookID, submissionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if submission.Status == BookSubmissionStatusApproved {
		tx.Rollback()
		return nil, errors.New("submission is already approved")
	}

	book, err := s.getBookModelTx(tx, bookID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	baseVersion, err := s.getBookVersionModelTx(tx, bookID, submission.BookVersionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	version, sectionIDMap, fieldIDMap, err := s.cloneVersionForApprovedSubmission(tx, baseVersion, userID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if submission.TargetSectionID == nil {
		if !version.AllowNewSections {
			tx.Rollback()
			return nil, errors.New("new sections are not enabled for this book version")
		}
		if strings.TrimSpace(stringValue(submission.NewSectionName)) == "" {
			tx.Rollback()
			return nil, errors.New("new_section_name is required")
		}

		sections, listErr := s.listVersionSectionModelsTx(tx, version.ID)
		if listErr != nil {
			tx.Rollback()
			return nil, listErr
		}
		if sectionNameExists(sections, stringValue(submission.NewSectionName), 0) {
			tx.Rollback()
			return nil, errors.New("new_section_name already exists")
		}

		nextSortOrder, nextErr := s.nextSectionSortOrder(tx, version.ID)
		if nextErr != nil {
			tx.Rollback()
			return nil, nextErr
		}

		section := BookVersionSection{
			BookVersionID: version.ID,
			Name:          strings.TrimSpace(stringValue(submission.NewSectionName)),
			SortOrder:     nextSortOrder,
		}
		if err := tx.Create(&section).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		submission.TargetSectionID = cloneInt(section.ID)
	} else {
		mappedSectionID, ok := sectionIDMap[*submission.TargetSectionID]
		if !ok {
			tx.Rollback()
			return nil, errors.New("target section does not belong to the approved book version")
		}
		submission.TargetSectionID = cloneInt(mappedSectionID)
	}

	if err := s.remapSubmissionValueFieldIDsTx(tx, submission.ID, fieldIDMap); err != nil {
		tx.Rollback()
		return nil, err
	}

	now := booksNowFunc()
	*submission = buildApprovedSubmissionRecord(*submission, version.ID, submission.TargetSectionID, userID, now)

	if err := tx.Save(submission).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.recomputeVersionSectionPageBounds(tx, *version); err != nil {
		tx.Rollback()
		return nil, err
	}

	sourcePDFContent, _, err := s.downloadStoredBookObject(storedBookObjectRef{
		ObjectKey: version.SourcePDFObjectKey,
		FileURL:   coalesceString(version.SourcePDFStorageURI, version.SourcePDFFileURL),
	}, ErrBookPDFNotFound)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	sections, err := s.listVersionSectionModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	fields, err := s.listVersionFieldModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	approvedSubmissions, err := s.listApprovedSubmissionModelsTx(tx, version.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	approvedSubmissionIDs := make([]int, 0, len(approvedSubmissions))
	for _, approvedSubmission := range approvedSubmissions {
		approvedSubmissionIDs = append(approvedSubmissionIDs, approvedSubmission.ID)
	}
	valuesBySubmission, err := s.listSubmissionValuesBySubmissionIDsTx(tx, approvedSubmissionIDs)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	imagesBySubmission, err := s.loadSubmissionImages(approvedSubmissions)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	generatedUpload, err := s.generateAndStoreVersionPDF(bookID, version, sourcePDFContent, sections, fields, approvedSubmissions, valuesBySubmission, imagesBySubmission)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	uploadedObjects := []string{}
	if generatedUpload.UploadedKey != "" {
		uploadedObjects = append(uploadedObjects, generatedUpload.UploadedKey)
	}
	applyStoredGeneratedPDFUpload(version, generatedUpload, now)
	if err := tx.Model(&BookVersion{}).Where("id = ?", version.ID).Updates(map[string]any{
		"generated_pdf_file_name":   nullableString(version.GeneratedPDFFileName),
		"generated_pdf_file_url":    nullableString(version.GeneratedPDFFileURL),
		"generated_pdf_storage_uri": nullableString(version.GeneratedPDFStorageURI),
		"generated_pdf_object_key":  nullableString(version.GeneratedPDFObjectKey),
		"last_generated_at":         version.LastGeneratedAt,
	}).Error; err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}
	if err := tx.Model(&Book{}).Where("id = ?", book.ID).Updates(map[string]any{
		"active_version_id": version.ID,
		"updated_by":        nullableInt(userID),
	}).Error; err != nil {
		tx.Rollback()
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupUploadedBookObjects(uploadedObjects)
		return nil, err
	}

	return &BookSubmissionMutationResponse{
		ID:                submission.ID,
		Status:            submission.Status,
		BookVersionID:     submission.BookVersionID,
		BookVersionNumber: version.VersionNumber,
		UpdatedAt:         submission.UpdatedAt,
	}, nil
}

func (s *BookService) RejectBookSubmission(bookID int, submissionID int, req ReviewBookSubmissionRequest) (*BookSubmissionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req.RejectionReason = strings.TrimSpace(req.RejectionReason)
	if req.RejectionReason == "" {
		return nil, errors.New("rejection_reason is required")
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackBooksOnPanic(tx)

	submission, err := s.getSubmissionModelTx(tx, bookID, submissionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if submission.Status == BookSubmissionStatusApproved {
		tx.Rollback()
		return nil, errors.New("approved submissions cannot be rejected")
	}
	book, err := s.getBookModelTx(tx, bookID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	version, err := s.getBookVersionModelTx(tx, bookID, submission.BookVersionID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	now := booksNowFunc()
	submission.Status = BookSubmissionStatusRejected
	submission.ReviewedBy = cloneIntPointer(req.ReviewedBy)
	submission.ReviewedAt = &now
	submission.RejectionReason = req.RejectionReason

	if err := tx.Save(submission).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	s.sendSubmitterRejectionEmail(book, version, submission)

	return &BookSubmissionMutationResponse{
		ID:        submission.ID,
		Status:    submission.Status,
		UpdatedAt: submission.UpdatedAt,
	}, nil
}

func (s *BookService) ListPublicBooks() ([]PublicBookSummaryResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var books []Book
	if err := s.DB.Where("active_version_id IS NOT NULL").Order("title ASC").Order("id ASC").Find(&books).Error; err != nil {
		return nil, err
	}

	resp := make([]PublicBookSummaryResponse, 0, len(books))
	for _, book := range books {
		resp = append(resp, PublicBookSummaryResponse{
			ID:              book.ID,
			Title:           strings.TrimSpace(book.Title),
			Description:     strings.TrimSpace(book.Description),
			ActiveVersionID: cloneIntPointer(book.ActiveVersionID),
		})
	}
	return resp, nil
}

func (s *BookService) GetPublicBook(bookID int) (*PublicBookDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	book, err := s.getBookModel(bookID)
	if err != nil {
		return nil, err
	}
	if book.ActiveVersionID == nil || *book.ActiveVersionID <= 0 {
		return nil, ErrBookActiveVersionNotFound
	}
	version, err := s.getBookVersionModel(bookID, *book.ActiveVersionID)
	if err != nil {
		return nil, err
	}
	sections, err := s.listVersionSectionResponses(version.ID)
	if err != nil {
		return nil, err
	}
	fields, err := s.listVersionFieldResponses(version.ID)
	if err != nil {
		return nil, err
	}

	return &PublicBookDetailResponse{
		ID:          book.ID,
		Title:       strings.TrimSpace(book.Title),
		Description: strings.TrimSpace(book.Description),
		Version: PublicBookActiveVersionResponse{
			ID:               version.ID,
			VersionNumber:    version.VersionNumber,
			PDFContentURL:    buildPublicPDFFetchURL(book.ID),
			AllowPageImage:   version.AllowPageImage,
			AllowNewSections: version.AllowNewSections,
			Sections:         sections,
			Fields:           fields,
		},
	}, nil
}

func (s *BookService) GetPublicActivePDFContent(bookID int) (*BookPDFContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	book, err := s.getBookModel(bookID)
	if err != nil {
		return nil, err
	}
	if book.ActiveVersionID == nil || *book.ActiveVersionID <= 0 {
		return nil, ErrBookActiveVersionNotFound
	}
	version, err := s.getBookVersionModel(bookID, *book.ActiveVersionID)
	if err != nil {
		return nil, err
	}

	if hasStoredBookFile(version.GeneratedPDFObjectKey, version.GeneratedPDFStorageURI, version.GeneratedPDFFileURL) {
		return s.downloadVersionPDF(version.GeneratedPDFObjectKey, coalesceString(version.GeneratedPDFStorageURI, version.GeneratedPDFFileURL), chooseNonEmpty(version.GeneratedPDFFileName, version.SourcePDFFileName))
	}
	return s.downloadVersionPDF(version.SourcePDFObjectKey, coalesceString(version.SourcePDFStorageURI, version.SourcePDFFileURL), version.SourcePDFFileName)
}

func normalizeSaveBookRequest(req SaveBookRequest) (SaveBookRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.AdminNotificationEmails = normalizeBookEmailList(req.AdminNotificationEmails)

	if req.Title == "" {
		return req, errors.New("title is required")
	}
	return req, nil
}

func validateManualInitialVersionNumber(nextVersionNumber int) error {
	if nextVersionNumber != 1 {
		return errors.New("initial version can only be created manually once per book")
	}
	return nil
}

func normalizeSaveBookVersionRequest(req SaveBookVersionRequest, requireSourcePDF bool) (SaveBookVersionRequest, error) {
	if req.SourcePageCount <= 0 {
		return req, errors.New("source_page_count must be a positive integer")
	}
	if req.ContentTemplatePageNumber <= 0 {
		return req, errors.New("content_template_page_number must be a positive integer")
	}
	if req.SectionTemplatePageNumber <= 0 {
		return req, errors.New("section_template_page_number must be a positive integer")
	}
	if req.ContentTemplatePageNumber > req.SourcePageCount {
		return req, errors.New("content_template_page_number must be within source_page_count")
	}
	if req.SectionTemplatePageNumber > req.SourcePageCount {
		return req, errors.New("section_template_page_number must be within source_page_count")
	}

	req.LayoutSettings = buildInitialBookLayoutSettings()

	if len(req.Sections) == 0 {
		return req, errors.New("sections is required")
	}
	if len(req.Fields) == 0 {
		return req, errors.New("fields is required")
	}

	seenGeneratedOnly := false
	previousEnd := 0
	seenSectionNames := make(map[string]int)
	for idx := range req.Sections {
		req.Sections[idx].Name = strings.TrimSpace(req.Sections[idx].Name)
		if req.Sections[idx].Name == "" {
			return req, fmt.Errorf("sections[%d].name is required", idx)
		}
		normalizedKey := strings.ToLower(req.Sections[idx].Name)
		if existingIdx, exists := seenSectionNames[normalizedKey]; exists {
			return req, fmt.Errorf("sections[%d].name duplicates sections[%d].name", idx, existingIdx)
		}
		seenSectionNames[normalizedKey] = idx

		start := req.Sections[idx].SourceStartPage
		end := req.Sections[idx].SourceEndPage
		switch {
		case start == nil && end == nil:
			seenGeneratedOnly = true
		case start == nil || end == nil:
			return req, fmt.Errorf("sections[%d] must include both source_start_page and source_end_page or neither", idx)
		default:
			if seenGeneratedOnly {
				return req, fmt.Errorf("sections[%d] must not define source pages after generated-only sections", idx)
			}
			if *start <= 0 || *end <= 0 {
				return req, fmt.Errorf("sections[%d] source pages must be positive integers", idx)
			}
			if *end < *start {
				return req, fmt.Errorf("sections[%d].source_end_page must be greater than or equal to source_start_page", idx)
			}
			if *end > req.SourcePageCount {
				return req, fmt.Errorf("sections[%d].source_end_page must be within source_page_count", idx)
			}
			if previousEnd > 0 && *start <= previousEnd {
				return req, fmt.Errorf("sections[%d] overlaps or is out of order", idx)
			}
			previousEnd = *end
		}
	}

	emailFieldCount := 0
	seenFieldLabels := make(map[string]int, len(req.Fields))
	for idx := range req.Fields {
		req.Fields[idx].Label = strings.TrimSpace(req.Fields[idx].Label)
		if req.Fields[idx].Label == "" {
			return req, fmt.Errorf("fields[%d].label is required", idx)
		}
		normalizedLabel := strings.ToLower(req.Fields[idx].Label)
		if existingIdx, exists := seenFieldLabels[normalizedLabel]; exists {
			return req, fmt.Errorf("fields[%d].label duplicates fields[%d].label", idx, existingIdx)
		}
		seenFieldLabels[normalizedLabel] = idx
		req.Fields[idx].InputType = strings.TrimSpace(strings.ToLower(req.Fields[idx].InputType))
		req.Fields[idx].Placement = strings.TrimSpace(strings.ToLower(req.Fields[idx].Placement))
		if !isAllowedBookValue(req.Fields[idx].InputType, BookFieldInputTypeSingleLine, BookFieldInputTypeRichText) {
			return req, fmt.Errorf("fields[%d].input_type is invalid", idx)
		}
		if !isAllowedBookValue(req.Fields[idx].Placement, BookFieldPlacementHeading, BookFieldPlacementBody) {
			return req, fmt.Errorf("fields[%d].placement is invalid", idx)
		}
		if req.Fields[idx].IsEmailField {
			if req.Fields[idx].InputType != BookFieldInputTypeSingleLine {
				return req, fmt.Errorf("fields[%d].input_type must be single_line when is_email_field is true", idx)
			}
			emailFieldCount++
		}
	}
	if emailFieldCount > 1 {
		return req, errors.New("only one field can be marked as is_email_field")
	}

	if req.SourcePDF != nil {
		cleaned := sanitizeBookUploadInput(*req.SourcePDF)
		req.SourcePDF = &cleaned
	}
	req.GeneratedPDF = nil

	if requireSourcePDF {
		if req.SourcePDF == nil || isEmptyBookUploadInput(*req.SourcePDF) {
			return req, errors.New("source_pdf is required")
		}
	}
	if req.SourcePDF != nil && !isEmptyBookUploadInput(*req.SourcePDF) {
		if err := validatePDFUploadInput(*req.SourcePDF, "source_pdf"); err != nil {
			return req, err
		}
	}
	return req, nil
}

func buildInitialBookLayoutSettings() json.RawMessage {
	return cloneRawJSON(defaultBookLayoutSettings)
}

func normalizeBookLayoutSettings(raw json.RawMessage) (json.RawMessage, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return cloneRawJSON(defaultBookLayoutSettings), nil
	}

	defaults, err := decodeBookLayoutSettings(defaultBookLayoutSettings)
	if err != nil {
		return nil, errors.New("layout_settings defaults are invalid")
	}

	decoded, err := decodeBookLayoutSettings(raw)
	if err != nil {
		return nil, errors.New("layout_settings must be valid JSON")
	}
	if decoded == nil {
		return nil, errors.New("layout_settings must be a JSON object")
	}
	if len(decoded) == 0 {
		return cloneRawJSON(defaultBookLayoutSettings), nil
	}

	normalized, err := json.Marshal(mergeBookLayoutSettings(defaults, decoded))
	if err != nil {
		return nil, errors.New("layout_settings must be valid JSON")
	}
	return normalized, nil
}

func decodeBookLayoutSettings(raw json.RawMessage) (map[string]any, error) {
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func mergeBookLayoutSettings(base map[string]any, overrides map[string]any) map[string]any {
	merged := cloneJSONObject(base)
	for key, overrideValue := range overrides {
		overrideMap, overrideIsMap := overrideValue.(map[string]any)
		baseMap, baseIsMap := merged[key].(map[string]any)
		if overrideIsMap && baseIsMap {
			merged[key] = mergeBookLayoutSettings(baseMap, overrideMap)
			continue
		}
		merged[key] = overrideValue
	}
	return merged
}

func cloneJSONObject(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		nested, ok := value.(map[string]any)
		if ok {
			cloned[key] = cloneJSONObject(nested)
			continue
		}
		cloned[key] = value
	}
	return cloned
}

func normalizeBookSubmissionRequest(req SaveBookSubmissionRequest, version BookVersion, fields []BookVersionField, sections []BookVersionSection) (SaveBookSubmissionRequest, string, []BookSubmissionValue, error) {
	req.NewSectionName = strings.TrimSpace(req.NewSectionName)

	if req.Image != nil {
		cleaned := sanitizeBookUploadInput(*req.Image)
		req.Image = &cleaned
	}

	switch {
	case req.TargetSectionID != nil && *req.TargetSectionID <= 0:
		return req, "", nil, errors.New("target_section_id must be a positive integer")
	case req.TargetSectionID != nil && req.NewSectionName != "":
		return req, "", nil, errors.New("new_section_name must be empty when target_section_id is provided")
	case req.TargetSectionID == nil && req.NewSectionName == "":
		return req, "", nil, errors.New("either target_section_id or new_section_name is required")
	}

	if req.TargetSectionID != nil {
		if !sectionIDExists(sections, *req.TargetSectionID) {
			return req, "", nil, errors.New("target_section_id references a section that does not exist")
		}
	} else {
		if !version.AllowNewSections {
			return req, "", nil, errors.New("new sections are not enabled for this book version")
		}
		if sectionNameExists(sections, req.NewSectionName, 0) {
			return req, "", nil, errors.New("new_section_name already exists")
		}
	}

	if req.Image != nil && !isEmptyBookUploadInput(*req.Image) {
		if !version.AllowPageImage {
			return req, "", nil, errors.New("image uploads are not enabled for this book version")
		}
		if err := validateImageUploadInput(*req.Image, "image"); err != nil {
			return req, "", nil, err
		}
	}

	if len(req.FieldValues) == 0 && len(fields) > 0 {
		requiredFieldExists := false
		for _, field := range fields {
			if field.IsRequired {
				requiredFieldExists = true
				break
			}
		}
		if requiredFieldExists {
			return req, "", nil, errors.New("field_values is required")
		}
	}

	fieldMap := make(map[int]BookVersionField, len(fields))
	for _, field := range fields {
		fieldMap[field.ID] = field
	}
	seenFieldIDs := make(map[int]struct{}, len(req.FieldValues))
	valueMap := make(map[int]string, len(req.FieldValues))
	for _, item := range req.FieldValues {
		if item.FieldID <= 0 {
			return req, "", nil, errors.New("field_values.field_id must be a positive integer")
		}
		if _, exists := seenFieldIDs[item.FieldID]; exists {
			return req, "", nil, fmt.Errorf("field_values contains duplicate field_id %d", item.FieldID)
		}
		if _, exists := fieldMap[item.FieldID]; !exists {
			return req, "", nil, fmt.Errorf("field_values references unknown field_id %d", item.FieldID)
		}
		seenFieldIDs[item.FieldID] = struct{}{}
		valueMap[item.FieldID] = strings.TrimSpace(item.Value)
	}

	submitterEmail := ""
	values := make([]BookSubmissionValue, 0, len(fields))
	for _, field := range fields {
		value := valueMap[field.ID]
		if field.IsRequired && value == "" {
			return req, "", nil, fmt.Errorf("%s is required", strings.ToLower(strings.ReplaceAll(field.Label, " ", "_")))
		}
		if field.IsEmailField && value != "" {
			if _, err := mail.ParseAddress(value); err != nil {
				return req, "", nil, fmt.Errorf("%s must be a valid email address", strings.ToLower(strings.ReplaceAll(field.Label, " ", "_")))
			}
			submitterEmail = value
		}
		if value == "" {
			continue
		}
		values = append(values, BookSubmissionValue{
			BookFieldID: field.ID,
			Value:       value,
		})
	}

	return req, submitterEmail, values, nil
}

func (s *BookService) listVersionSectionResponses(versionID int) ([]BookVersionSectionResponse, error) {
	sections, err := s.listVersionSectionModels(versionID)
	if err != nil {
		return nil, err
	}

	resp := make([]BookVersionSectionResponse, 0, len(sections))
	for _, section := range sections {
		resp = append(resp, BookVersionSectionResponse{
			ID:               section.ID,
			Name:             strings.TrimSpace(section.Name),
			SourceStartPage:  cloneIntPointer(section.SourceStartPage),
			SourceEndPage:    cloneIntPointer(section.SourceEndPage),
			CurrentStartPage: section.CurrentStartPage,
			CurrentEndPage:   section.CurrentEndPage,
			SortOrder:        section.SortOrder,
			CreatedAt:        section.CreatedAt,
			UpdatedAt:        section.UpdatedAt,
		})
	}
	if resp == nil {
		resp = []BookVersionSectionResponse{}
	}
	return resp, nil
}

func (s *BookService) listVersionFieldResponses(versionID int) ([]BookVersionFieldResponse, error) {
	fields, err := s.listVersionFieldModels(versionID)
	if err != nil {
		return nil, err
	}

	resp := make([]BookVersionFieldResponse, 0, len(fields))
	for _, field := range fields {
		resp = append(resp, BookVersionFieldResponse{
			ID:           field.ID,
			Label:        strings.TrimSpace(field.Label),
			InputType:    field.InputType,
			Placement:    field.Placement,
			ShowLabel:    field.ShowLabel,
			IsRequired:   field.IsRequired,
			IsEmailField: field.IsEmailField,
			SortOrder:    field.SortOrder,
			CreatedAt:    field.CreatedAt,
			UpdatedAt:    field.UpdatedAt,
		})
	}
	if resp == nil {
		resp = []BookVersionFieldResponse{}
	}
	return resp, nil
}

func (s *BookService) listSubmissionResponses(bookID int, filter ListBookSubmissionsFilter, orderClause string) ([]BookSubmissionResponse, error) {
	query := s.DB.Where("book_id = ?", bookID)
	if filter.VersionID > 0 {
		query = query.Where("book_version_id = ?", filter.VersionID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if strings.TrimSpace(orderClause) == "" {
		orderClause = "created_at DESC, id DESC"
	}

	var submissions []BookSubmission
	if err := query.Order(orderClause).Find(&submissions).Error; err != nil {
		return nil, err
	}
	return s.buildSubmissionResponses(submissions)
}

func (s *BookService) buildSubmissionResponses(submissions []BookSubmission) ([]BookSubmissionResponse, error) {
	if len(submissions) == 0 {
		return []BookSubmissionResponse{}, nil
	}

	submissionIDs := make([]int, 0, len(submissions))
	versionIDSet := make(map[int]struct{}, len(submissions))
	sectionIDSet := make(map[int]struct{}, len(submissions))
	for _, submission := range submissions {
		submissionIDs = append(submissionIDs, submission.ID)
		versionIDSet[submission.BookVersionID] = struct{}{}
		if submission.TargetSectionID != nil {
			sectionIDSet[*submission.TargetSectionID] = struct{}{}
		}
	}

	versionIDs := mapKeysInt(versionIDSet)
	sectionIDs := mapKeysInt(sectionIDSet)

	versionsByID, err := s.versionMapByID(versionIDs)
	if err != nil {
		return nil, err
	}
	sectionsByID, err := s.sectionMapByID(sectionIDs)
	if err != nil {
		return nil, err
	}

	var values []BookSubmissionValue
	if err := s.DB.Where("book_submission_id IN ?", submissionIDs).Find(&values).Error; err != nil {
		return nil, err
	}
	valuesBySubmission := make(map[int][]BookSubmissionValue, len(submissions))
	fieldIDSet := make(map[int]struct{}, len(values))
	for _, value := range values {
		valuesBySubmission[value.BookSubmissionID] = append(valuesBySubmission[value.BookSubmissionID], value)
		fieldIDSet[value.BookFieldID] = struct{}{}
	}

	fieldIDs := mapKeysInt(fieldIDSet)
	fieldsByID, err := s.fieldMapByID(fieldIDs)
	if err != nil {
		return nil, err
	}

	resp := make([]BookSubmissionResponse, 0, len(submissions))
	for _, submission := range submissions {
		fieldValues := valuesBySubmission[submission.ID]
		sort.SliceStable(fieldValues, func(i, j int) bool {
			leftField := fieldsByID[fieldValues[i].BookFieldID]
			rightField := fieldsByID[fieldValues[j].BookFieldID]
			if leftField.SortOrder != rightField.SortOrder {
				return leftField.SortOrder < rightField.SortOrder
			}
			return leftField.ID < rightField.ID
		})

		fieldResponses := make([]BookSubmissionValueResponse, 0, len(fieldValues))
		for _, value := range fieldValues {
			field := fieldsByID[value.BookFieldID]
			fieldResponses = append(fieldResponses, BookSubmissionValueResponse{
				FieldID:      field.ID,
				Label:        strings.TrimSpace(field.Label),
				InputType:    field.InputType,
				Placement:    field.Placement,
				ShowLabel:    field.ShowLabel,
				IsEmailField: field.IsEmailField,
				Value:        value.Value,
			})
		}
		if fieldResponses == nil {
			fieldResponses = []BookSubmissionValueResponse{}
		}

		targetSectionName := strings.TrimSpace(stringValue(submission.NewSectionName))
		if submission.TargetSectionID != nil {
			if section, ok := sectionsByID[*submission.TargetSectionID]; ok {
				targetSectionName = strings.TrimSpace(section.Name)
			}
		}

		var image *BookSubmissionImageResponse
		if hasStoredBookFile(submission.ImageObjectKey, submission.ImageStorageURI, submission.ImageFileURL) {
			image = &BookSubmissionImageResponse{
				FileName: strings.TrimSpace(submission.ImageFileName),
				MimeType: strings.TrimSpace(submission.ImageMimeType),
				FileSize: submission.ImageFileSize,
				FetchURL: buildSubmissionImageFetchURL(submission.BookID, submission.ID),
			}
		}

		versionNumber := 0
		if version, ok := versionsByID[submission.BookVersionID]; ok {
			versionNumber = version.VersionNumber
		}

		resp = append(resp, BookSubmissionResponse{
			ID:                submission.ID,
			BookID:            submission.BookID,
			BookVersionID:     submission.BookVersionID,
			BookVersionNumber: versionNumber,
			TargetSectionID:   cloneIntPointer(submission.TargetSectionID),
			TargetSectionName: targetSectionName,
			NewSectionName:    strings.TrimSpace(stringValue(submission.NewSectionName)),
			Status:            submission.Status,
			SubmitterEmail:    strings.TrimSpace(submission.SubmitterEmail),
			Image:             image,
			FieldValues:       fieldResponses,
			ReviewedBy:        cloneIntPointer(submission.ReviewedBy),
			ReviewedAt:        cloneTimePointer(submission.ReviewedAt),
			RejectionReason:   strings.TrimSpace(submission.RejectionReason),
			CreatedAt:         submission.CreatedAt,
			UpdatedAt:         submission.UpdatedAt,
		})
	}

	return resp, nil
}

func (s *BookService) versionCountByBook(bookIDs []int) (map[int]int, error) {
	if len(bookIDs) == 0 {
		return map[int]int{}, nil
	}
	type row struct {
		BookID int   `gorm:"column:book_id"`
		Count  int64 `gorm:"column:count"`
	}

	var rows []row
	if err := s.DB.Model(&BookVersion{}).
		Select("book_id, COUNT(*) AS count").
		Where("book_id IN ?", bookIDs).
		Group("book_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	resp := make(map[int]int, len(rows))
	for _, row := range rows {
		resp[row.BookID] = int(row.Count)
	}
	return resp, nil
}

func (s *BookService) pendingSubmissionCountByBook(bookIDs []int) (map[int]int, error) {
	if len(bookIDs) == 0 {
		return map[int]int{}, nil
	}
	type row struct {
		BookID int   `gorm:"column:book_id"`
		Count  int64 `gorm:"column:count"`
	}
	var rows []row
	if err := s.DB.Model(&BookSubmission{}).
		Select("book_id, COUNT(*) AS count").
		Where("book_id IN ? AND status = ?", bookIDs, BookSubmissionStatusPending).
		Group("book_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	resp := make(map[int]int, len(rows))
	for _, row := range rows {
		resp[row.BookID] = int(row.Count)
	}
	return resp, nil
}

func (s *BookService) versionNumberByID(versionIDs []int) (map[int]int, error) {
	if len(versionIDs) == 0 {
		return map[int]int{}, nil
	}
	var versions []BookVersion
	if err := s.DB.Select("id", "version_number").Where("id IN ?", versionIDs).Find(&versions).Error; err != nil {
		return nil, err
	}
	resp := make(map[int]int, len(versions))
	for _, version := range versions {
		resp[version.ID] = version.VersionNumber
	}
	return resp, nil
}

func (s *BookService) sectionCountByVersion(versionIDs []int) (map[int]int, error) {
	if len(versionIDs) == 0 {
		return map[int]int{}, nil
	}
	var rows []bookSectionCountRow
	if err := s.DB.Model(&BookVersionSection{}).
		Select("book_version_id, COUNT(*) AS count").
		Where("book_version_id IN ?", versionIDs).
		Group("book_version_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	resp := make(map[int]int, len(rows))
	for _, row := range rows {
		resp[row.BookVersionID] = int(row.Count)
	}
	return resp, nil
}

func (s *BookService) fieldCountByVersion(versionIDs []int) (map[int]int, error) {
	if len(versionIDs) == 0 {
		return map[int]int{}, nil
	}
	var rows []bookFieldCountRow
	if err := s.DB.Model(&BookVersionField{}).
		Select("book_version_id, COUNT(*) AS count").
		Where("book_version_id IN ?", versionIDs).
		Group("book_version_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	resp := make(map[int]int, len(rows))
	for _, row := range rows {
		resp[row.BookVersionID] = int(row.Count)
	}
	return resp, nil
}

type versionSubmissionCounts struct {
	Approved int
	Pending  int
	Rejected int
}

func (s *BookService) submissionCountByVersion(versionIDs []int) (map[int]versionSubmissionCounts, error) {
	if len(versionIDs) == 0 {
		return map[int]versionSubmissionCounts{}, nil
	}
	var rows []bookVersionSubmissionCountRow
	if err := s.DB.Model(&BookSubmission{}).
		Select("book_version_id, status, COUNT(*) AS count").
		Where("book_version_id IN ?", versionIDs).
		Group("book_version_id, status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	resp := make(map[int]versionSubmissionCounts, len(rows))
	for _, row := range rows {
		counts := resp[row.BookVersionID]
		switch row.Status {
		case BookSubmissionStatusApproved:
			counts.Approved = int(row.Count)
		case BookSubmissionStatusPending:
			counts.Pending = int(row.Count)
		case BookSubmissionStatusRejected:
			counts.Rejected = int(row.Count)
		}
		resp[row.BookVersionID] = counts
	}
	return resp, nil
}

func (s *BookService) getBookModel(bookID int) (*Book, error) {
	return s.getBookModelTx(s.DB, bookID)
}

func (s *BookService) getBookModelTx(tx *gorm.DB, bookID int) (*Book, error) {
	var book Book
	if err := tx.First(&book, bookID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}
	return &book, nil
}

func (s *BookService) getBookVersionModel(bookID int, versionID int) (*BookVersion, error) {
	return s.getBookVersionModelTx(s.DB, bookID, versionID)
}

func (s *BookService) getBookVersionModelTx(tx *gorm.DB, bookID int, versionID int) (*BookVersion, error) {
	var version BookVersion
	if err := tx.Where("book_id = ? AND id = ?", bookID, versionID).First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookVersionNotFound
		}
		return nil, err
	}
	return &version, nil
}

func (s *BookService) getSubmissionModel(bookID int, submissionID int) (*BookSubmission, error) {
	return s.getSubmissionModelTx(s.DB, bookID, submissionID)
}

func (s *BookService) getSubmissionModelTx(tx *gorm.DB, bookID int, submissionID int) (*BookSubmission, error) {
	var submission BookSubmission
	if err := tx.Where("book_id = ? AND id = ?", bookID, submissionID).First(&submission).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookSubmissionNotFound
		}
		return nil, err
	}
	return &submission, nil
}

func (s *BookService) nextVersionNumber(tx *gorm.DB, bookID int) (int, error) {
	type row struct {
		MaxVersionNumber int `gorm:"column:max_version_number"`
	}
	var result row
	if err := tx.Model(&BookVersion{}).
		Select("COALESCE(MAX(version_number), 0) AS max_version_number").
		Where("book_id = ?", bookID).
		Scan(&result).Error; err != nil {
		return 0, err
	}
	return result.MaxVersionNumber + 1, nil
}

func (s *BookService) syncVersionSections(tx *gorm.DB, versionID int, sourcePageCount int, reqSections []SaveBookVersionSectionRequest) error {
	var existing []BookVersionSection
	if err := tx.Where("book_version_id = ?", versionID).Find(&existing).Error; err != nil {
		return err
	}
	existingByID := make(map[int]BookVersionSection, len(existing))
	for _, item := range existing {
		existingByID[item.ID] = item
	}

	referencedSectionIDs, err := s.referencedSectionIDs(tx, versionID)
	if err != nil {
		return err
	}
	seenIDs := make(map[int]struct{}, len(reqSections))

	for idx, req := range reqSections {
		section := BookVersionSection{
			BookVersionID:    versionID,
			Name:             strings.TrimSpace(req.Name),
			SourceStartPage:  cloneIntPointer(req.SourceStartPage),
			SourceEndPage:    cloneIntPointer(req.SourceEndPage),
			CurrentStartPage: 0,
			CurrentEndPage:   0,
			SortOrder:        idx,
		}

		if req.ID > 0 {
			existingSection, ok := existingByID[req.ID]
			if !ok {
				return fmt.Errorf("section id %d does not belong to this book version", req.ID)
			}
			seenIDs[req.ID] = struct{}{}
			existingSection.Name = section.Name
			existingSection.SourceStartPage = section.SourceStartPage
			existingSection.SourceEndPage = section.SourceEndPage
			existingSection.SortOrder = idx
			if err := tx.Save(&existingSection).Error; err != nil {
				return err
			}
			continue
		}

		if err := tx.Create(&section).Error; err != nil {
			return err
		}
	}

	for _, section := range existing {
		if _, seen := seenIDs[section.ID]; seen {
			continue
		}
		if _, referenced := referencedSectionIDs[section.ID]; referenced {
			return fmt.Errorf("section %q cannot be removed because submissions reference it", section.Name)
		}
		if err := tx.Delete(&BookVersionSection{}, section.ID).Error; err != nil {
			return err
		}
	}

	return s.validateStoredVersionSections(tx, versionID, sourcePageCount)
}

func (s *BookService) syncVersionFields(tx *gorm.DB, versionID int, reqFields []SaveBookVersionFieldRequest) error {
	var existing []BookVersionField
	if err := tx.Where("book_version_id = ?", versionID).Find(&existing).Error; err != nil {
		return err
	}
	existingByID := make(map[int]BookVersionField, len(existing))
	for _, item := range existing {
		existingByID[item.ID] = item
	}

	referencedFieldIDs, err := s.referencedFieldIDs(tx, versionID)
	if err != nil {
		return err
	}
	seenIDs := make(map[int]struct{}, len(reqFields))

	for idx, req := range reqFields {
		field := BookVersionField{
			BookVersionID: versionID,
			Label:         strings.TrimSpace(req.Label),
			InputType:     strings.TrimSpace(strings.ToLower(req.InputType)),
			Placement:     strings.TrimSpace(strings.ToLower(req.Placement)),
			ShowLabel:     req.ShowLabel,
			IsRequired:    req.IsRequired,
			IsEmailField:  req.IsEmailField,
			SortOrder:     idx,
		}

		if req.ID > 0 {
			existingField, ok := existingByID[req.ID]
			if !ok {
				return fmt.Errorf("field id %d does not belong to this book version", req.ID)
			}
			seenIDs[req.ID] = struct{}{}
			existingField.Label = field.Label
			existingField.InputType = field.InputType
			existingField.Placement = field.Placement
			existingField.ShowLabel = field.ShowLabel
			existingField.IsRequired = field.IsRequired
			existingField.IsEmailField = field.IsEmailField
			existingField.SortOrder = idx
			if err := tx.Save(&existingField).Error; err != nil {
				return err
			}
			continue
		}

		if err := tx.Create(&field).Error; err != nil {
			return err
		}
	}

	for _, field := range existing {
		if _, seen := seenIDs[field.ID]; seen {
			continue
		}
		if _, referenced := referencedFieldIDs[field.ID]; referenced {
			return fmt.Errorf("field %q cannot be removed because submissions reference it", field.Label)
		}
		if err := tx.Delete(&BookVersionField{}, field.ID).Error; err != nil {
			return err
		}
	}

	return s.validateStoredVersionFields(tx, versionID)
}

func (s *BookService) validateStoredVersionSections(tx *gorm.DB, versionID int, sourcePageCount int) error {
	sections, err := s.listVersionSectionModelsTx(tx, versionID)
	if err != nil {
		return err
	}

	seenGeneratedOnly := false
	previousEnd := 0
	seenNames := make(map[string]int, len(sections))
	for idx, section := range sections {
		key := strings.ToLower(strings.TrimSpace(section.Name))
		if previousIdx, exists := seenNames[key]; exists {
			return fmt.Errorf("section %q duplicates section at position %d", section.Name, previousIdx+1)
		}
		seenNames[key] = idx

		switch {
		case section.SourceStartPage == nil && section.SourceEndPage == nil:
			seenGeneratedOnly = true
		case section.SourceStartPage == nil || section.SourceEndPage == nil:
			return fmt.Errorf("section %q must include both source_start_page and source_end_page or neither", section.Name)
		default:
			if seenGeneratedOnly {
				return fmt.Errorf("section %q cannot appear after generated-only sections", section.Name)
			}
			if *section.SourceStartPage <= 0 || *section.SourceEndPage <= 0 {
				return fmt.Errorf("section %q must use positive source pages", section.Name)
			}
			if *section.SourceEndPage < *section.SourceStartPage {
				return fmt.Errorf("section %q has an invalid page range", section.Name)
			}
			if *section.SourceEndPage > sourcePageCount {
				return fmt.Errorf("section %q exceeds source_page_count", section.Name)
			}
			if previousEnd > 0 && *section.SourceStartPage <= previousEnd {
				return fmt.Errorf("section %q overlaps another section", section.Name)
			}
			previousEnd = *section.SourceEndPage
		}
	}

	return nil
}

func (s *BookService) validateStoredVersionFields(tx *gorm.DB, versionID int) error {
	fields, err := s.listVersionFieldModelsTx(tx, versionID)
	if err != nil {
		return err
	}

	emailFieldCount := 0
	seenLabels := make(map[string]int, len(fields))
	for idx, field := range fields {
		normalizedLabel := strings.ToLower(strings.TrimSpace(field.Label))
		if normalizedLabel == "" {
			return fmt.Errorf("field at position %d is missing a label", idx+1)
		}
		if previousIdx, exists := seenLabels[normalizedLabel]; exists {
			return fmt.Errorf("field %q duplicates field at position %d", field.Label, previousIdx+1)
		}
		seenLabels[normalizedLabel] = idx
	}
	for _, field := range fields {
		if !isAllowedBookValue(field.InputType, BookFieldInputTypeSingleLine, BookFieldInputTypeRichText) {
			return fmt.Errorf("field %q has an invalid input_type", field.Label)
		}
		if !isAllowedBookValue(field.Placement, BookFieldPlacementHeading, BookFieldPlacementBody) {
			return fmt.Errorf("field %q has an invalid placement", field.Label)
		}
		if field.IsEmailField {
			if field.InputType != BookFieldInputTypeSingleLine {
				return fmt.Errorf("field %q must use input_type %q when is_email_field is true", field.Label, BookFieldInputTypeSingleLine)
			}
			emailFieldCount++
		}
	}
	if emailFieldCount > 1 {
		return errors.New("only one field can be marked as is_email_field")
	}
	return nil
}

func (s *BookService) recomputeVersionSectionPageBounds(tx *gorm.DB, version BookVersion) error {
	sections, err := s.listVersionSectionModelsTx(tx, version.ID)
	if err != nil {
		return err
	}
	if len(sections) == 0 {
		return nil
	}

	countsBySection := make(map[int]int)
	var countRows []bookSubmissionSectionCountRow
	if err := tx.Model(&BookSubmission{}).
		Select("target_section_id, COUNT(*) AS count").
		Where("book_version_id = ? AND status = ? AND target_section_id IS NOT NULL", version.ID, BookSubmissionStatusApproved).
		Group("target_section_id").
		Scan(&countRows).Error; err != nil {
		return err
	}
	for _, row := range countRows {
		countsBySection[row.TargetSectionID] = int(row.Count)
	}

	sourceCursor := 1
	currentCursor := 1

	for _, section := range sections {
		startPage := section.SourceStartPage
		endPage := section.SourceEndPage
		addedPages := countsBySection[section.ID]

		if startPage != nil && endPage != nil {
			gapCount := 0
			if sourceCursor < *startPage {
				gapCount = *startPage - sourceCursor
			}
			baseCount := (*endPage - *startPage) + 1
			currentStart := currentCursor + gapCount
			currentEnd := currentStart + baseCount + addedPages - 1

			if err := tx.Model(&BookVersionSection{}).
				Where("id = ?", section.ID).
				Updates(map[string]any{
					"current_start_page": currentStart,
					"current_end_page":   currentEnd,
				}).Error; err != nil {
				return err
			}

			currentCursor = currentEnd + 1
			sourceCursor = *endPage + 1
			continue
		}

		if sourceCursor <= version.SourcePageCount {
			currentCursor += (version.SourcePageCount - sourceCursor) + 1
			sourceCursor = version.SourcePageCount + 1
		}

		currentStart := currentCursor
		currentEnd := currentStart + 1 + addedPages - 1
		if err := tx.Model(&BookVersionSection{}).
			Where("id = ?", section.ID).
			Updates(map[string]any{
				"current_start_page": currentStart,
				"current_end_page":   currentEnd,
			}).Error; err != nil {
			return err
		}
		currentCursor = currentEnd + 1
	}

	return nil
}

func (s *BookService) referencedSectionIDs(tx *gorm.DB, versionID int) (map[int]struct{}, error) {
	type row struct {
		TargetSectionID int `gorm:"column:target_section_id"`
	}
	var rows []row
	if err := tx.Model(&BookSubmission{}).
		Select("DISTINCT target_section_id").
		Where("book_version_id = ? AND target_section_id IS NOT NULL", versionID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	resp := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		resp[row.TargetSectionID] = struct{}{}
	}
	return resp, nil
}

func (s *BookService) referencedFieldIDs(tx *gorm.DB, versionID int) (map[int]struct{}, error) {
	type row struct {
		BookFieldID int `gorm:"column:book_field_id"`
	}
	var rows []row
	if err := tx.Table("book_submission_values AS bsv").
		Select("DISTINCT bsv.book_field_id").
		Joins("INNER JOIN book_submissions AS bs ON bs.id = bsv.book_submission_id").
		Where("bs.book_version_id = ?", versionID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	resp := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		resp[row.BookFieldID] = struct{}{}
	}
	return resp, nil
}

func (s *BookService) nextSectionSortOrder(tx *gorm.DB, versionID int) (int, error) {
	type row struct {
		MaxSortOrder int `gorm:"column:max_sort_order"`
	}
	var result row
	if err := tx.Model(&BookVersionSection{}).
		Select("COALESCE(MAX(sort_order), -1) AS max_sort_order").
		Where("book_version_id = ?", versionID).
		Scan(&result).Error; err != nil {
		return 0, err
	}
	return result.MaxSortOrder + 1, nil
}

func (s *BookService) listVersionSectionModels(versionID int) ([]BookVersionSection, error) {
	return s.listVersionSectionModelsTx(s.DB, versionID)
}

func (s *BookService) listVersionSectionModelsTx(tx *gorm.DB, versionID int) ([]BookVersionSection, error) {
	var sections []BookVersionSection
	if err := tx.Where("book_version_id = ?", versionID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&sections).Error; err != nil {
		return nil, err
	}
	if sections == nil {
		sections = []BookVersionSection{}
	}
	return sections, nil
}

func (s *BookService) listVersionFieldModels(versionID int) ([]BookVersionField, error) {
	return s.listVersionFieldModelsTx(s.DB, versionID)
}

func (s *BookService) listVersionFieldModelsTx(tx *gorm.DB, versionID int) ([]BookVersionField, error) {
	var fields []BookVersionField
	if err := tx.Where("book_version_id = ?", versionID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&fields).Error; err != nil {
		return nil, err
	}
	if fields == nil {
		fields = []BookVersionField{}
	}
	return fields, nil
}

func (s *BookService) fieldMapByVersion(versionIDs []int) (map[int][]BookVersionField, error) {
	resp := make(map[int][]BookVersionField)
	if len(versionIDs) == 0 {
		return resp, nil
	}
	var fields []BookVersionField
	if err := s.DB.Where("book_version_id IN ?", versionIDs).
		Order("book_version_id ASC").
		Order("sort_order ASC").
		Order("id ASC").
		Find(&fields).Error; err != nil {
		return nil, err
	}
	for _, field := range fields {
		resp[field.BookVersionID] = append(resp[field.BookVersionID], field)
	}
	return resp, nil
}

func (s *BookService) fieldMapByID(fieldIDs []int) (map[int]BookVersionField, error) {
	resp := make(map[int]BookVersionField)
	if len(fieldIDs) == 0 {
		return resp, nil
	}
	var fields []BookVersionField
	if err := s.DB.Where("id IN ?", fieldIDs).Find(&fields).Error; err != nil {
		return nil, err
	}
	for _, field := range fields {
		resp[field.ID] = field
	}
	return resp, nil
}

func (s *BookService) sectionMapByID(sectionIDs []int) (map[int]BookVersionSection, error) {
	resp := make(map[int]BookVersionSection)
	if len(sectionIDs) == 0 {
		return resp, nil
	}
	var sections []BookVersionSection
	if err := s.DB.Where("id IN ?", sectionIDs).Find(&sections).Error; err != nil {
		return nil, err
	}
	for _, section := range sections {
		resp[section.ID] = section
	}
	return resp, nil
}

func (s *BookService) versionMapByID(versionIDs []int) (map[int]BookVersion, error) {
	resp := make(map[int]BookVersion)
	if len(versionIDs) == 0 {
		return resp, nil
	}
	var versions []BookVersion
	if err := s.DB.Where("id IN ?", versionIDs).Find(&versions).Error; err != nil {
		return nil, err
	}
	for _, version := range versions {
		resp[version.ID] = version
	}
	return resp, nil
}

func (s *BookService) cloneVersionForApprovedSubmission(tx *gorm.DB, sourceVersion *BookVersion, userID *int) (*BookVersion, map[int]int, map[int]int, error) {
	versionNumber, err := s.nextVersionNumber(tx, sourceVersion.BookID)
	if err != nil {
		return nil, nil, nil, err
	}

	nextVersion := buildApprovedVersionClone(*sourceVersion, versionNumber, userID)
	clonedVersion := &nextVersion
	if err := tx.Create(clonedVersion).Error; err != nil {
		return nil, nil, nil, err
	}

	sections, err := s.listVersionSectionModelsTx(tx, sourceVersion.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	sectionIDMap := make(map[int]int, len(sections))
	for _, section := range sections {
		clonedSection := BookVersionSection{
			BookVersionID:    clonedVersion.ID,
			Name:             section.Name,
			SourceStartPage:  cloneIntPointer(section.SourceStartPage),
			SourceEndPage:    cloneIntPointer(section.SourceEndPage),
			CurrentStartPage: 0,
			CurrentEndPage:   0,
			SortOrder:        section.SortOrder,
		}
		if err := tx.Create(&clonedSection).Error; err != nil {
			return nil, nil, nil, err
		}
		sectionIDMap[section.ID] = clonedSection.ID
	}

	fields, err := s.listVersionFieldModelsTx(tx, sourceVersion.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	fieldIDMap := make(map[int]int, len(fields))
	for _, field := range fields {
		clonedField := BookVersionField{
			BookVersionID: clonedVersion.ID,
			Label:         field.Label,
			InputType:     field.InputType,
			Placement:     field.Placement,
			ShowLabel:     field.ShowLabel,
			IsRequired:    field.IsRequired,
			IsEmailField:  field.IsEmailField,
			SortOrder:     field.SortOrder,
		}
		if err := tx.Create(&clonedField).Error; err != nil {
			return nil, nil, nil, err
		}
		fieldIDMap[field.ID] = clonedField.ID
	}

	if err := s.cloneApprovedSubmissionsForVersionTx(tx, sourceVersion.ID, clonedVersion.ID, sectionIDMap, fieldIDMap); err != nil {
		return nil, nil, nil, err
	}

	return clonedVersion, sectionIDMap, fieldIDMap, nil
}

func (s *BookService) cloneApprovedSubmissionsForVersionTx(tx *gorm.DB, sourceVersionID int, targetVersionID int, sectionIDMap map[int]int, fieldIDMap map[int]int) error {
	submissions, err := s.listApprovedSubmissionModelsTx(tx, sourceVersionID)
	if err != nil {
		return err
	}
	if len(submissions) == 0 {
		return nil
	}

	submissionIDs := make([]int, 0, len(submissions))
	for _, submission := range submissions {
		submissionIDs = append(submissionIDs, submission.ID)
	}
	valuesBySubmission, err := s.listSubmissionValuesBySubmissionIDsTx(tx, submissionIDs)
	if err != nil {
		return err
	}

	for _, submission := range submissions {
		clonedSubmission := submission
		clonedSubmission.ID = 0
		clonedSubmission.BookVersionID = targetVersionID
		if submission.TargetSectionID != nil {
			mappedSectionID, ok := sectionIDMap[*submission.TargetSectionID]
			if !ok {
				return fmt.Errorf("section id %d does not belong to the approved book version", *submission.TargetSectionID)
			}
			clonedSubmission.TargetSectionID = cloneInt(mappedSectionID)
		} else {
			clonedSubmission.TargetSectionID = nil
		}

		if err := tx.Create(&clonedSubmission).Error; err != nil {
			return err
		}

		for _, value := range valuesBySubmission[submission.ID] {
			nextFieldID, ok := fieldIDMap[value.BookFieldID]
			if !ok {
				return fmt.Errorf("field id %d does not belong to the approved book version", value.BookFieldID)
			}
			clonedValue := value
			clonedValue.ID = 0
			clonedValue.BookSubmissionID = clonedSubmission.ID
			clonedValue.BookFieldID = nextFieldID
			if err := tx.Create(&clonedValue).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *BookService) remapSubmissionValueFieldIDsTx(tx *gorm.DB, submissionID int, fieldIDMap map[int]int) error {
	if len(fieldIDMap) == 0 {
		return nil
	}

	var values []BookSubmissionValue
	if err := tx.Where("book_submission_id = ?", submissionID).Find(&values).Error; err != nil {
		return err
	}
	for _, value := range values {
		nextFieldID, ok := fieldIDMap[value.BookFieldID]
		if !ok {
			return fmt.Errorf("field id %d does not belong to the approved book version", value.BookFieldID)
		}
		if err := tx.Model(&BookSubmissionValue{}).
			Where("id = ?", value.ID).
			Update("book_field_id", nextFieldID).Error; err != nil {
			return err
		}
	}
	return nil
}

func buildApprovedVersionClone(sourceVersion BookVersion, versionNumber int, userID *int) BookVersion {
	return BookVersion{
		BookID:                    sourceVersion.BookID,
		VersionNumber:             versionNumber,
		SourcePageCount:           sourceVersion.SourcePageCount,
		ContentTemplatePageNumber: sourceVersion.ContentTemplatePageNumber,
		SectionTemplatePageNumber: sourceVersion.SectionTemplatePageNumber,
		AllowPageImage:            sourceVersion.AllowPageImage,
		AllowNewSections:          sourceVersion.AllowNewSections,
		LayoutSettings:            cloneRawJSON(sourceVersion.LayoutSettings),
		SourcePDFFileName:         sourceVersion.SourcePDFFileName,
		SourcePDFFileURL:          sourceVersion.SourcePDFFileURL,
		SourcePDFStorageURI:       sourceVersion.SourcePDFStorageURI,
		SourcePDFObjectKey:        sourceVersion.SourcePDFObjectKey,
		CreatedBy:                 cloneIntPointer(userID),
		UpdatedBy:                 cloneIntPointer(userID),
	}
}

func buildApprovedSubmissionRecord(submission BookSubmission, versionID int, targetSectionID *int, userID *int, reviewedAt time.Time) BookSubmission {
	submission.BookVersionID = versionID
	submission.TargetSectionID = cloneIntPointer(targetSectionID)
	submission.Status = BookSubmissionStatusApproved
	submission.ReviewedBy = cloneIntPointer(userID)
	submission.ReviewedAt = cloneTimePointer(&reviewedAt)
	submission.RejectionReason = ""
	return submission
}

func (s *BookService) listApprovedSubmissionModelsTx(tx *gorm.DB, versionID int) ([]BookSubmission, error) {
	var submissions []BookSubmission
	if err := tx.Where("book_version_id = ? AND status = ?", versionID, BookSubmissionStatusApproved).
		Order("reviewed_at ASC NULLS LAST").
		Order("id ASC").
		Find(&submissions).Error; err != nil {
		return nil, err
	}
	if submissions == nil {
		submissions = []BookSubmission{}
	}
	return submissions, nil
}

func (s *BookService) listSubmissionValuesBySubmissionIDsTx(tx *gorm.DB, submissionIDs []int) (map[int][]BookSubmissionValue, error) {
	valuesBySubmission := make(map[int][]BookSubmissionValue)
	if len(submissionIDs) == 0 {
		return valuesBySubmission, nil
	}

	var values []BookSubmissionValue
	if err := tx.Where("book_submission_id IN ?", submissionIDs).
		Order("id ASC").
		Find(&values).Error; err != nil {
		return nil, err
	}
	for _, value := range values {
		valuesBySubmission[value.BookSubmissionID] = append(valuesBySubmission[value.BookSubmissionID], value)
	}
	return valuesBySubmission, nil
}

func (s *BookService) loadSubmissionImages(submissions []BookSubmission) (map[int]*storedBookUploadContent, error) {
	imagesBySubmission := make(map[int]*storedBookUploadContent)
	for _, submission := range submissions {
		if !hasStoredBookFile(submission.ImageObjectKey, submission.ImageStorageURI, submission.ImageFileURL) {
			continue
		}

		data, contentType, err := s.downloadStoredBookObject(storedBookObjectRef{
			ObjectKey: submission.ImageObjectKey,
			FileURL:   coalesceString(submission.ImageStorageURI, submission.ImageFileURL),
		}, ErrBookSubmissionImageNotFound)
		if err != nil {
			return nil, err
		}
		imagesBySubmission[submission.ID] = &storedBookUploadContent{
			Data:     data,
			MimeType: chooseNonEmpty(strings.TrimSpace(contentType), strings.TrimSpace(submission.ImageMimeType)),
			FileName: strings.TrimSpace(submission.ImageFileName),
		}
	}
	return imagesBySubmission, nil
}

func (s *BookService) resolveBookUploadContent(input *BookUploadInput, stored storedBookUpload, notFoundErr error) ([]byte, error) {
	if input != nil && len(input.Content) > 0 {
		return append([]byte(nil), input.Content...), nil
	}
	data, _, err := s.downloadStoredBookObject(storedBookObjectRef{
		ObjectKey: stored.ObjectKey,
		FileURL:   coalesceString(stored.StorageURI, stored.FileURL),
	}, notFoundErr)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *BookService) generateAndStoreVersionPDF(bookID int, version *BookVersion, sourcePDF []byte, sections []BookVersionSection, fields []BookVersionField, submissions []BookSubmission, valuesBySubmission map[int][]BookSubmissionValue, imagesBySubmission map[int]*storedBookUploadContent) (storedBookUpload, error) {
	generatedPDF, err := generateBookVersionPDF(sourcePDF, *version, sections, fields, submissions, valuesBySubmission, imagesBySubmission)
	if err != nil {
		return storedBookUpload{}, err
	}

	return s.storeVersionPDF(bookID, version.VersionNumber, "generated", BookUploadInput{
		FileName: buildGeneratedPDFFileName(version.SourcePDFFileName),
		MimeType: "application/pdf",
		FileSize: int64(len(generatedPDF)),
		Content:  generatedPDF,
	})
}

func applyStoredGeneratedPDFUpload(version *BookVersion, upload storedBookUpload, generatedAt time.Time) {
	version.GeneratedPDFFileName = upload.FileName
	version.GeneratedPDFFileURL = upload.FileURL
	version.GeneratedPDFStorageURI = upload.StorageURI
	version.GeneratedPDFObjectKey = upload.ObjectKey
	version.LastGeneratedAt = cloneTimePointer(&generatedAt)
}

func buildGeneratedPDFFileName(sourceFileName string) string {
	baseName := strings.TrimSpace(path.Base(sourceFileName))
	baseName = strings.TrimSpace(strings.TrimSuffix(baseName, path.Ext(baseName)))
	if baseName == "" {
		baseName = "book"
	}
	return baseName + "_generated.pdf"
}

func (s *BookService) storeVersionPDF(bookID int, versionNumber int, folder string, input BookUploadInput) (storedBookUpload, error) {
	input = sanitizeBookUploadInput(input)
	if err := validatePDFUploadInput(input, folder+"_pdf"); err != nil {
		return storedBookUpload{}, err
	}
	return s.storeBookUpload(input, s.buildVersionPDFObjectKey(bookID, versionNumber, folder, input.FileName, input.MimeType), "book-"+folder)
}

func (s *BookService) storeSubmissionImage(bookID int, submissionID int, input BookUploadInput) (storedBookUpload, error) {
	input = sanitizeBookUploadInput(input)
	if err := validateImageUploadInput(input, "image"); err != nil {
		return storedBookUpload{}, err
	}
	return s.storeBookUpload(input, s.buildSubmissionImageObjectKey(bookID, submissionID, input.FileName, input.MimeType), "submission-image")
}

func (s *BookService) storeBookUpload(input BookUploadInput, objectKey string, fallbackFileName string) (storedBookUpload, error) {
	fileName := strings.TrimSpace(input.FileName)
	mimeType := strings.TrimSpace(input.MimeType)
	fileURL := strings.TrimSpace(input.FileURL)
	storageURI := strings.TrimSpace(input.StorageURI)
	objectRef := strings.TrimSpace(input.ObjectKey)
	if objectRef == "" {
		objectRef = strings.TrimSpace(input.GCPObjectKey)
	}
	fileSize := input.FileSize

	if len(input.Content) == 0 {
		reference := coalesceString(storageURI, fileURL)
		if reference == "" && objectRef == "" {
			return storedBookUpload{}, errors.New("uploaded file content or a storage reference is required")
		}
		if fileName == "" {
			fileName = buildStoredBookFileName(objectRef, fallbackFileName)
		}
		if objectRef == "" && looksLikeBookGCSReference(reference) {
			_, parsedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), reference)
			if err == nil {
				objectRef = parsedObjectKey
			}
		}
		if storageURI == "" {
			storageURI = reference
		}
		if fileURL == "" {
			fileURL = reference
		}
		return storedBookUpload{
			FileName:   fileName,
			FileURL:    fileURL,
			StorageURI: storageURI,
			ObjectKey:  objectRef,
			MimeType:   mimeType,
			FileSize:   fileSize,
		}, nil
	}

	if strings.TrimSpace(s.BucketName) == "" {
		return storedBookUpload{}, ErrMediaBucketNotConfigured
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(input.Content)
	}
	if fileName == "" {
		fileName = fallbackFileName + util.ExtFromFilenameOrMime(fileName, mimeType)
	}
	if fileSize <= 0 {
		fileSize = int64(len(input.Content))
	}
	if strings.TrimSpace(objectKey) == "" {
		return storedBookUpload{}, errors.New("storage object key is required")
	}

	uploadedURI, uploadedSize, err := bookUploadBytesToGCSHook(input.Content, s.BucketName, objectKey, mimeType)
	if err != nil {
		return storedBookUpload{}, err
	}
	if uploadedSize > 0 {
		fileSize = uploadedSize
	}

	return storedBookUpload{
		FileName:    fileName,
		FileURL:     uploadedURI,
		StorageURI:  uploadedURI,
		ObjectKey:   objectKey,
		MimeType:    mimeType,
		FileSize:    fileSize,
		UploadedKey: objectKey,
	}, nil
}

func (s *BookService) buildVersionPDFObjectKey(bookID int, versionNumber int, folder string, fileName string, mimeType string) string {
	return s.buildBookObjectKey(
		path.Join("books", fmt.Sprintf("book-%d", bookID), "versions", fmt.Sprintf("version-%d", versionNumber), folder),
		fileName,
		mimeType,
		folder,
	)
}

func (s *BookService) buildSubmissionImageObjectKey(bookID int, submissionID int, fileName string, mimeType string) string {
	return s.buildBookObjectKey(
		path.Join("books", fmt.Sprintf("book-%d", bookID), "submissions", fmt.Sprintf("submission-%d", submissionID), "images"),
		fileName,
		mimeType,
		"image",
	)
}

func (s *BookService) buildBookObjectKey(baseFolder string, fileName string, mimeType string, fallbackBase string) string {
	prefix := strings.Trim(strings.TrimSpace(s.BucketPrefix), "/")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = fallbackBase
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	if ext == "" && strings.Contains(strings.ToLower(strings.TrimSpace(mimeType)), "pdf") {
		ext = ".pdf"
	}
	timestamp := booksNowFunc().UTC().Format("20060102150405")
	key := path.Join(baseFolder, fmt.Sprintf("%s_%s%s", timestamp, base, ext))
	if prefix == "" {
		return key
	}
	return path.Join(prefix, key)
}

func (s *BookService) downloadVersionPDF(objectKey string, storageURI string, fallbackFileName string) (*BookPDFContent, error) {
	if !hasStoredBookFile(objectKey, storageURI, storageURI) {
		return nil, ErrBookPDFNotFound
	}
	data, contentType, err := s.downloadStoredBookObject(storedBookObjectRef{
		ObjectKey: objectKey,
		FileURL:   storageURI,
	}, ErrBookPDFNotFound)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/pdf"
	}
	fileName := strings.TrimSpace(fallbackFileName)
	if fileName == "" {
		fileName = buildStoredBookFileName(objectKey, "book.pdf")
	}
	return &BookPDFContent{
		Content:     data,
		ContentType: contentType,
		FileName:    fileName,
	}, nil
}

func (s *BookService) downloadStoredBookObject(ref storedBookObjectRef, notFoundErr error) ([]byte, string, error) {
	bucketName, objectKey, err := s.resolveStoredBookObjectReference(ref)
	if err != nil {
		if errors.Is(err, util.ErrBucketNameRequired) {
			return nil, "", ErrMediaBucketNotConfigured
		}
		if errors.Is(err, util.ErrObjectNameRequired) {
			return nil, "", notFoundErr
		}
		return nil, "", err
	}

	data, contentType, err := bookDownloadGCSObjectHook(bucketName, objectKey)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, "", notFoundErr
		}
		return nil, "", err
	}
	return data, contentType, nil
}

func (s *BookService) resolveStoredBookObjectReference(ref storedBookObjectRef) (string, string, error) {
	objectKey := strings.TrimSpace(ref.ObjectKey)
	fileURL := strings.TrimSpace(ref.FileURL)
	if objectKey != "" && strings.TrimSpace(s.BucketName) != "" {
		return strings.TrimSpace(s.BucketName), objectKey, nil
	}
	if fileURL == "" {
		if objectKey != "" {
			return "", "", util.ErrBucketNameRequired
		}
		return "", "", util.ErrObjectNameRequired
	}
	return util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), fileURL)
}

func (s *BookService) cleanupUploadedBookObjects(objectKeys []string) {
	for _, objectKey := range objectKeys {
		objectKey = strings.TrimSpace(objectKey)
		if objectKey == "" || strings.TrimSpace(s.BucketName) == "" {
			continue
		}
		_ = bookDeleteGCSObjectHook(strings.TrimSpace(s.BucketName), objectKey)
	}
}

func (s *BookService) cleanupStoredBookObjectsBestEffort(items []storedBookObjectRef) {
	for _, item := range items {
		bucketName, objectKey, err := s.resolveStoredBookObjectReference(item)
		if err != nil || strings.TrimSpace(bucketName) == "" || strings.TrimSpace(objectKey) == "" {
			continue
		}
		_ = bookDeleteGCSObjectHook(bucketName, objectKey)
	}
}

func (s *BookService) sendAdminNewSubmissionEmail(book *Book, version *BookVersion, submission BookSubmission, sections []BookVersionSection) {
	if s.EmailSender == nil || book == nil {
		return
	}
	recipients := normalizeBookEmailList(cloneStringSlice([]string(book.AdminNotificationEmails)))
	if len(recipients) == 0 {
		return
	}

	sectionName := strings.TrimSpace(stringValue(submission.NewSectionName))
	if submission.TargetSectionID != nil {
		for _, section := range sections {
			if section.ID == *submission.TargetSectionID {
				sectionName = strings.TrimSpace(section.Name)
				break
			}
		}
	}
	if sectionName == "" {
		sectionName = "Unknown section"
	}

	subject := fmt.Sprintf("New book submission for %s", strings.TrimSpace(book.Title))
	body := fmt.Sprintf(
		"A new submission is waiting for review.\n\nBook: %s\nVersion: %d\nSubmission ID: %d\nSection: %s\nSubmitted at: %s\nSubmitter email: %s\n",
		strings.TrimSpace(book.Title),
		version.VersionNumber,
		submission.ID,
		sectionName,
		submission.CreatedAt.Format(time.RFC3339),
		chooseNonEmpty(strings.TrimSpace(submission.SubmitterEmail), "Not provided"),
	)

	go s.sendEmailBestEffort(recipients, subject, body)
}

func (s *BookService) sendSubmitterRejectionEmail(book *Book, version *BookVersion, submission *BookSubmission) {
	if s.EmailSender == nil || submission == nil {
		return
	}
	email := strings.TrimSpace(submission.SubmitterEmail)
	if email == "" {
		return
	}
	subject := fmt.Sprintf("Your %s submission needs updates", strings.TrimSpace(book.Title))
	body := fmt.Sprintf(
		"Your submission for %s could not be approved yet.\n\nBook: %s\nVersion: %d\nSubmission ID: %d\nReason: %s\n\nPlease review the details and submit again if needed.\n",
		strings.TrimSpace(book.Title),
		strings.TrimSpace(book.Title),
		version.VersionNumber,
		submission.ID,
		strings.TrimSpace(submission.RejectionReason),
	)

	go s.sendEmailBestEffort([]string{email}, subject, body)
}

func (s *BookService) sendEmailBestEffort(to []string, subject string, body string) {
	if s.EmailSender == nil || len(to) == 0 {
		return
	}
	if err := s.EmailSender.SendEmail(to, subject, body); err != nil {
		log.Printf("books email send failed: recipients=%v subject=%q err=%v", to, subject, err)
	}
}

func normalizeBookEmailList(emails []string) []string {
	seen := make(map[string]struct{}, len(emails))
	resp := make([]string, 0, len(emails))
	for _, email := range emails {
		email = strings.TrimSpace(strings.ToLower(email))
		if email == "" {
			continue
		}
		if _, err := mail.ParseAddress(email); err != nil {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		resp = append(resp, email)
	}
	return resp
}

func sanitizeBookUploadInput(input BookUploadInput) BookUploadInput {
	input.FileName = strings.TrimSpace(input.FileName)
	input.MimeType = strings.TrimSpace(strings.ToLower(input.MimeType))
	input.FileURL = strings.TrimSpace(input.FileURL)
	input.StorageURI = strings.TrimSpace(input.StorageURI)
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.GCPObjectKey = strings.TrimSpace(input.GCPObjectKey)
	if input.FileSize <= 0 && len(input.Content) > 0 {
		input.FileSize = int64(len(input.Content))
	}
	if input.MimeType == "" && len(input.Content) > 0 {
		input.MimeType = strings.TrimSpace(strings.ToLower(http.DetectContentType(input.Content)))
	}
	return input
}

func isEmptyBookUploadInput(input BookUploadInput) bool {
	return strings.TrimSpace(input.FileName) == "" &&
		strings.TrimSpace(input.MimeType) == "" &&
		strings.TrimSpace(input.FileURL) == "" &&
		strings.TrimSpace(input.StorageURI) == "" &&
		strings.TrimSpace(input.ObjectKey) == "" &&
		strings.TrimSpace(input.GCPObjectKey) == "" &&
		input.FileSize == 0 &&
		len(input.Content) == 0
}

func validatePDFUploadInput(input BookUploadInput, fieldName string) error {
	if len(input.Content) == 0 && strings.TrimSpace(input.FileURL) == "" && strings.TrimSpace(input.StorageURI) == "" && strings.TrimSpace(input.ObjectKey) == "" && strings.TrimSpace(input.GCPObjectKey) == "" {
		return fmt.Errorf("%s is required", fieldName)
	}

	mimeType := strings.ToLower(strings.TrimSpace(input.MimeType))
	fileName := strings.TrimSpace(input.FileName)
	objectKey := chooseNonEmpty(strings.TrimSpace(input.ObjectKey), strings.TrimSpace(input.GCPObjectKey))
	reference := chooseNonEmpty(strings.TrimSpace(input.StorageURI), strings.TrimSpace(input.FileURL), objectKey, fileName)

	switch {
	case mimeType == "application/pdf":
		return nil
	case strings.HasSuffix(strings.ToLower(fileName), ".pdf"):
		return nil
	case strings.HasSuffix(strings.ToLower(objectKey), ".pdf"):
		return nil
	case strings.HasSuffix(strings.ToLower(reference), ".pdf"):
		return nil
	default:
		return fmt.Errorf("%s must be a PDF", fieldName)
	}
}

func validateImageUploadInput(input BookUploadInput, fieldName string) error {
	if len(input.Content) == 0 && strings.TrimSpace(input.FileURL) == "" && strings.TrimSpace(input.StorageURI) == "" && strings.TrimSpace(input.ObjectKey) == "" && strings.TrimSpace(input.GCPObjectKey) == "" {
		return fmt.Errorf("%s is required", fieldName)
	}

	mimeType := strings.ToLower(strings.TrimSpace(input.MimeType))
	fileName := strings.ToLower(strings.TrimSpace(input.FileName))
	objectKey := strings.ToLower(strings.TrimSpace(chooseNonEmpty(input.ObjectKey, input.GCPObjectKey)))
	reference := strings.ToLower(strings.TrimSpace(chooseNonEmpty(input.StorageURI, input.FileURL)))
	if strings.HasPrefix(mimeType, "image/") {
		return nil
	}
	if hasAllowedImageExt(fileName) || hasAllowedImageExt(objectKey) || hasAllowedImageExt(reference) {
		return nil
	}
	return fmt.Errorf("%s must be an image file", fieldName)
}

func hasAllowedImageExt(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasSuffix(value, ".jpg") ||
		strings.HasSuffix(value, ".jpeg") ||
		strings.HasSuffix(value, ".png") ||
		strings.HasSuffix(value, ".gif") ||
		strings.HasSuffix(value, ".webp")
}

func sectionIDExists(sections []BookVersionSection, sectionID int) bool {
	for _, section := range sections {
		if section.ID == sectionID {
			return true
		}
	}
	return false
}

func sectionNameExists(sections []BookVersionSection, name string, excludeID int) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return false
	}
	for _, section := range sections {
		if excludeID > 0 && section.ID == excludeID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(section.Name), target) {
			return true
		}
	}
	return false
}

func shouldCleanupStoredBookObject(previous storedBookObjectRef, next storedBookObjectRef) bool {
	return strings.TrimSpace(previous.ObjectKey) != "" &&
		(strings.TrimSpace(previous.ObjectKey) != strings.TrimSpace(next.ObjectKey) ||
			strings.TrimSpace(previous.FileURL) != strings.TrimSpace(next.FileURL))
}

func hasStoredBookFile(objectKey string, storageURI string, fileURL string) bool {
	return strings.TrimSpace(objectKey) != "" ||
		strings.TrimSpace(storageURI) != "" ||
		strings.TrimSpace(fileURL) != ""
}

func buildStoredBookFileName(objectKey string, fallback string) string {
	base := path.Base(strings.TrimSpace(objectKey))
	if base == "" || base == "." || base == "/" {
		return fallback
	}
	return base
}

func looksLikeBookGCSReference(fileURL string) bool {
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

func buildSourcePDFFetchURL(bookID int, versionID int) string {
	return fmt.Sprintf("/api/books/%d/versions/%d/source/content", bookID, versionID)
}

func buildGeneratedPDFFetchURL(bookID int, versionID int) string {
	return fmt.Sprintf("/api/books/%d/versions/%d/generated/content", bookID, versionID)
}

func buildSubmissionImageFetchURL(bookID int, submissionID int) string {
	return fmt.Sprintf("/api/books/%d/submissions/%d/image/content", bookID, submissionID)
}

func buildPublicPDFFetchURL(bookID int) string {
	return fmt.Sprintf("/api/books/public/%d/pdf/content", bookID)
}

func isAllowedBookValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func rollbackBooksOnPanic(tx *gorm.DB) {
	if recover() != nil {
		tx.Rollback()
		panic("transaction panic")
	}
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make([]byte, len(raw))
	copy(cloned, raw)
	return cloned
}

func cloneStringSlice(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	cloned := make([]string, len(input))
	copy(cloned, input)
	return cloned
}

func cloneInt(value int) *int {
	next := value
	return &next
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	next := *value
	return &next
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	next := *value
	return &next
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableStringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func nullableInt64Value(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func chooseNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func coalesceString(values ...string) string {
	return chooseNonEmpty(values...)
}

func mapKeysInt(input map[int]struct{}) []int {
	resp := make([]int, 0, len(input))
	for key := range input {
		resp = append(resp, key)
	}
	sort.Ints(resp)
	return resp
}

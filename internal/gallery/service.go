package gallery

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
	ErrStoreUnavailable         = errors.New("gallery store unavailable")
	ErrGalleryNotFound          = errors.New("gallery not found")
	ErrGalleryImageNotFound     = errors.New("gallery image not found")
	ErrMediaBucketNotConfigured = errors.New("drive bucket is not configured")
)

var (
	nowFunc               = time.Now
	uploadBase64ToGCSHook = func(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
		return util.UploadBase64ToGCS(base64Data, bucketName, objectName, contentType)
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error {
		return util.DeleteGCSObject(bucketName, objectName)
	}
)

type GalleryService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

func (s *GalleryService) CreateGallery(req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeSaveGalleryRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	uploadedObjects := make([]string, 0, 1)

	row := Gallery{
		Name:        req.Name,
		Description: req.Description,
		Published:   req.Published,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}
	if err := tx.Create(&row).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if req.CoverImage != nil {
		coverURL, coverObjectKey, _, uploadedObject, err := s.storeGalleryImage(row.ID, "cover", 0, *req.CoverImage)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}
		row.CoverImageURL = coverURL
		row.CoverImageObjectKey = coverObjectKey
		row.CoverImageAltText = req.CoverImage.AltText
		if err := tx.Save(&row).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &GalleryMutationResponse{ID: row.ID, Name: row.Name, Published: row.Published}, nil
}

func (s *GalleryService) UpdateGallery(id int, req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeSaveGalleryRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	uploadedObjects := make([]string, 0, 1)
	oldObjects := make([]galleryStoredObject, 0, 1)

	var row Gallery
	if err := tx.First(&row, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryNotFound
		}
		return nil, err
	}

	row.Name = req.Name
	row.Description = req.Description
	row.Published = req.Published
	row.UpdatedBy = userID

	if req.RemoveCoverImage && row.CoverImageURL != "" {
		oldObjects = append(oldObjects, galleryStoredObject{ObjectKey: row.CoverImageObjectKey, StorageURL: row.CoverImageURL})
		row.CoverImageURL = ""
		row.CoverImageObjectKey = ""
		row.CoverImageAltText = ""
	}

	if req.CoverImage != nil {
		if row.CoverImageURL != "" {
			oldObjects = append(oldObjects, galleryStoredObject{ObjectKey: row.CoverImageObjectKey, StorageURL: row.CoverImageURL})
		}
		coverURL, coverObjectKey, _, uploadedObject, err := s.storeGalleryImage(row.ID, "cover", 0, *req.CoverImage)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}
		row.CoverImageURL = coverURL
		row.CoverImageObjectKey = coverObjectKey
		row.CoverImageAltText = req.CoverImage.AltText
	}

	if err := tx.Save(&row).Error; err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.cleanupStoredObjects(oldObjects); err != nil {
		return nil, err
	}

	return &GalleryMutationResponse{ID: row.ID, Name: row.Name, Published: row.Published}, nil
}

func (s *GalleryService) DeleteGallery(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackOnPanic(tx)

	var row Gallery
	if err := tx.First(&row, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGalleryNotFound
		}
		return err
	}

	var images []GalleryImage
	if err := tx.Where("gallery_id = ?", id).Find(&images).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Delete(&row).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	toCleanup := make([]galleryStoredObject, 0, len(images)+1)
	if row.CoverImageURL != "" || row.CoverImageObjectKey != "" {
		toCleanup = append(toCleanup, galleryStoredObject{ObjectKey: row.CoverImageObjectKey, StorageURL: row.CoverImageURL})
	}
	for _, img := range images {
		toCleanup = append(toCleanup, galleryStoredObject{ObjectKey: img.GCPObjectKey, StorageURL: img.FileURL})
	}
	return s.cleanupStoredObjects(toCleanup)
}

func (s *GalleryService) AddGalleryImages(id int, req AddGalleryImagesRequest, userID *int) (*DeleteGalleryImagesResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeAddGalleryImagesRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	var galleryRow Gallery
	if err := tx.First(&galleryRow, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryNotFound
		}
		return nil, err
	}

	uploadedObjects := make([]string, 0, len(req.Images))
	for idx, input := range req.Images {
		fileURL, objectKey, fileSize, uploadedObject, err := s.storeGalleryImage(id, "images", idx, input)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}

		row := GalleryImage{
			GalleryID:    id,
			AltText:      input.AltText,
			GCPObjectKey: objectKey,
			FileURL:      fileURL,
			MimeType:     input.MimeType,
			FileSize:     fileSize,
			UploadedBy:   userID,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &DeleteGalleryImagesResponse{DeletedCount: len(req.Images)}, nil
}

func (s *GalleryService) DeleteGalleryImages(id int, storageURLs []string) (*DeleteGalleryImagesResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	storageURLs = sanitizeStringSlice(storageURLs)
	if len(storageURLs) == 0 {
		return nil, errors.New("at least one storage_url is required")
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	var rows []GalleryImage
	if err := tx.Where("gallery_id = ? AND file_url IN ?", id, storageURLs).Find(&rows).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if len(rows) == 0 {
		tx.Rollback()
		return nil, ErrGalleryImageNotFound
	}

	if err := tx.Delete(&rows).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	toCleanup := make([]galleryStoredObject, 0, len(rows))
	for _, row := range rows {
		toCleanup = append(toCleanup, galleryStoredObject{ObjectKey: row.GCPObjectKey, StorageURL: row.FileURL})
	}
	if err := s.cleanupStoredObjects(toCleanup); err != nil {
		return nil, err
	}

	return &DeleteGalleryImagesResponse{DeletedCount: len(rows)}, nil
}

type galleryStoredObject struct {
	ObjectKey  string
	StorageURL string
}

func normalizeSaveGalleryRequest(req SaveGalleryRequest) (SaveGalleryRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		return req, errors.New("name is required")
	}
	if req.CoverImage != nil {
		cleaned := sanitizeGalleryUploadInput(*req.CoverImage)
		if err := validateGalleryUploadInput(cleaned); err != nil {
			return req, err
		}
		req.CoverImage = &cleaned
	}
	return req, nil
}

func normalizeAddGalleryImagesRequest(req AddGalleryImagesRequest) (AddGalleryImagesRequest, error) {
	if len(req.Images) == 0 {
		return req, errors.New("images are required")
	}
	cleaned := make([]GalleryUploadInput, 0, len(req.Images))
	for _, item := range req.Images {
		item = sanitizeGalleryUploadInput(item)
		if err := validateGalleryUploadInput(item); err != nil {
			return req, err
		}
		cleaned = append(cleaned, item)
	}
	req.Images = cleaned
	return req, nil
}

func sanitizeGalleryUploadInput(value GalleryUploadInput) GalleryUploadInput {
	value.AltText = strings.TrimSpace(value.AltText)
	value.FileName = strings.TrimSpace(value.FileName)
	value.MimeType = strings.TrimSpace(value.MimeType)
	value.DataBase64 = strings.TrimSpace(value.DataBase64)
	value.FileURL = strings.TrimSpace(value.FileURL)
	value.StorageURI = strings.TrimSpace(value.StorageURI)
	value.ObjectKey = strings.TrimSpace(value.ObjectKey)
	value.GCPObjectKey = strings.TrimSpace(value.GCPObjectKey)
	return value
}

func validateGalleryUploadInput(value GalleryUploadInput) error {
	if strings.TrimSpace(value.DataBase64) == "" && strings.TrimSpace(value.StorageURI) == "" && strings.TrimSpace(value.FileURL) == "" {
		return errors.New("image upload is missing both data_base64 and file_url")
	}
	mimeType := strings.ToLower(strings.TrimSpace(value.MimeType))
	if mimeType != "" && !strings.HasPrefix(mimeType, "image/") {
		return errors.New("only image uploads are supported")
	}
	if mimeType == "" {
		ext := strings.ToLower(util.ExtFromFilenameOrMime(value.FileName, value.MimeType))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		default:
			if strings.TrimSpace(value.DataBase64) != "" {
				return errors.New("only image uploads are supported")
			}
		}
	}
	return nil
}

func (s *GalleryService) storeGalleryImage(galleryID int, folder string, idx int, input GalleryUploadInput) (string, string, int64, string, error) {
	referenceURL := strings.TrimSpace(input.StorageURI)
	if referenceURL == "" {
		referenceURL = strings.TrimSpace(input.FileURL)
	}
	objectKey := strings.TrimSpace(input.ObjectKey)
	if objectKey == "" {
		objectKey = strings.TrimSpace(input.GCPObjectKey)
	}

	if strings.TrimSpace(input.DataBase64) == "" {
		if referenceURL == "" {
			return "", "", 0, "", errors.New("image upload is missing both data_base64 and file_url")
		}
		if objectKey == "" {
			_, resolvedObjectKey, err := util.ParseGCSObjectReference(s.BucketName, referenceURL)
			if err == nil {
				objectKey = s.relativeObjectKey(resolvedObjectKey)
			}
		}
		return referenceURL, objectKey, 0, "", nil
	}

	if strings.TrimSpace(s.BucketName) == "" {
		return "", "", 0, "", ErrMediaBucketNotConfigured
	}

	objectName := s.galleryObjectName(galleryID, folder, idx, input.FileName, input.MimeType)
	storageObjectName := s.storageObjectName(objectName)
	fileURL, fileSize, err := uploadBase64ToGCSHook(input.DataBase64, s.BucketName, storageObjectName, input.MimeType)
	if err != nil {
		return "", "", 0, "", err
	}

	return fileURL, objectName, fileSize, storageObjectName, nil
}

func (s *GalleryService) cleanupStoredObjects(items []galleryStoredObject) error {
	var cleanupErr error
	for _, item := range items {
		if err := s.cleanupStoredObject(item); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func (s *GalleryService) cleanupStoredObject(item galleryStoredObject) error {
	bucketName, objectName, err := s.resolveObjectReference(item)
	if err != nil {
		return err
	}
	if objectName == "" {
		return nil
	}
	return deleteGCSObjectHook(bucketName, objectName)
}

func (s *GalleryService) resolveObjectReference(item galleryStoredObject) (string, string, error) {
	objectKey := strings.TrimSpace(item.ObjectKey)
	if objectKey != "" {
		bucketName := strings.TrimSpace(s.BucketName)
		if bucketName == "" {
			return "", "", ErrMediaBucketNotConfigured
		}
		return bucketName, s.storageObjectName(objectKey), nil
	}
	if strings.TrimSpace(item.StorageURL) == "" {
		return "", "", nil
	}
	bucketName, objectName, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), item.StorageURL)
	if err != nil {
		if errors.Is(err, util.ErrBucketNameRequired) {
			return "", "", ErrMediaBucketNotConfigured
		}
		return "", "", err
	}
	if bucketName == "" {
		return "", "", ErrMediaBucketNotConfigured
	}
	return bucketName, objectName, nil
}

func (s *GalleryService) cleanupObjects(objectNames []string) {
	for _, objectName := range objectNames {
		if strings.TrimSpace(objectName) == "" || strings.TrimSpace(s.BucketName) == "" {
			continue
		}
		_ = deleteGCSObjectHook(s.BucketName, objectName)
	}
}

func (s *GalleryService) galleryObjectName(galleryID int, folder string, idx int, fileName string, mimeType string) string {
	timestamp := nowFunc().UTC().Format("20060102150405")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = "image"
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("galleries/%d/%s/%s_%d_%s%s", galleryID, folder, timestamp, idx+1, base, ext)
}

func (s *GalleryService) storageObjectName(objectKey string) string {
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

func (s *GalleryService) relativeObjectKey(objectKey string) string {
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

func sanitizeStringSlice(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func rollbackOnPanic(tx *gorm.DB) {
	if recover() != nil {
		tx.Rollback()
		panic("transaction panic")
	}
}

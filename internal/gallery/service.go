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

const defaultGalleryAssetLimit = 20

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
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return util.ReadGCSObject(bucketName, objectName)
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

func (s *GalleryService) ListGalleries() (*GalleryListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var rows []Gallery
	if err := s.DB.
		Order("LOWER(name) ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]GallerySummaryItem, 0, len(rows))
	if len(rows) == 0 {
		return &GalleryListResponse{Items: items}, nil
	}

	galleryIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		galleryIDs = append(galleryIDs, row.ID)
	}

	type galleryCountRow struct {
		GalleryID int
		Count     int64
	}
	var countRows []galleryCountRow
	if err := s.DB.
		Model(&GalleryImage{}).
		Select("gallery_id, COUNT(*) AS count").
		Where("gallery_id IN ?", galleryIDs).
		Group("gallery_id").
		Scan(&countRows).Error; err != nil {
		return nil, err
	}

	assetCountByGallery := make(map[int]int, len(countRows))
	for _, row := range countRows {
		assetCountByGallery[row.GalleryID] = int(row.Count)
	}

	var imageRows []GalleryImage
	if err := s.DB.
		Where("gallery_id IN ?", galleryIDs).
		Order("gallery_id ASC").
		Order("sort_order ASC").
		Order("id ASC").
		Find(&imageRows).Error; err != nil {
		return nil, err
	}

	firstImageByGallery := make(map[int]GalleryImage, len(imageRows))
	for _, row := range imageRows {
		if _, exists := firstImageByGallery[row.GalleryID]; !exists {
			firstImageByGallery[row.GalleryID] = row
		}
	}

	for _, row := range rows {
		frontImageURL := ""
		switch {
		case strings.TrimSpace(row.CoverImageURL) != "" || strings.TrimSpace(row.CoverImageObjectKey) != "":
			frontImageURL = buildGalleryCoverFetchURL(row.ID)
		default:
			if image, ok := firstImageByGallery[row.ID]; ok {
				frontImageURL = buildGalleryImageFetchURL(row.ID, image.ID)
			}
		}

		items = append(items, GallerySummaryItem{
			ID:            row.ID,
			Name:          row.Name,
			Published:     row.Published,
			AssetCount:    assetCountByGallery[row.ID],
			FrontImageURL: frontImageURL,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		})
	}

	return &GalleryListResponse{Items: items}, nil
}

func (s *GalleryService) GetGallery(id int) (*GalleryDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var row Gallery
	if err := s.DB.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryNotFound
		}
		return nil, err
	}

	var images []GalleryImage
	if err := s.DB.
		Where("gallery_id = ?", id).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&images).Error; err != nil {
		return nil, err
	}

	items := make([]GalleryAssetResponse, 0, len(images))
	for _, image := range images {
		items = append(items, mapGalleryAssetResponse(image))
	}

	return &GalleryDetailResponse{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Published:   row.Published,
		AssetLimit:  defaultGalleryAssetLimit,
		CoverImage:  mapGalleryCoverResponse(row),
		Images:      items,
		CreatedBy:   row.CreatedBy,
		UpdatedBy:   row.UpdatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

func (s *GalleryService) GetGalleryCoverContent(id int) (*GalleryMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var row Gallery
	if err := s.DB.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(row.CoverImageObjectKey) == "" && strings.TrimSpace(row.CoverImageURL) == "" {
		return nil, ErrGalleryImageNotFound
	}

	content, contentType, err := s.downloadStoredObject(galleryStoredObject{
		ObjectKey:  row.CoverImageObjectKey,
		StorageURL: row.CoverImageURL,
	})
	if err != nil {
		return nil, err
	}

	return &GalleryMediaContent{
		Content:     content,
		ContentType: contentType,
		FileName: buildGalleryContentFileName(
			row.CoverImageAltText,
			row.CoverImageObjectKey,
			row.CoverImageURL,
			contentType,
			"gallery-cover",
		),
	}, nil
}

func (s *GalleryService) GetGalleryImageContent(id int, imageID int) (*GalleryMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var row GalleryImage
	if err := s.DB.Where("gallery_id = ? AND id = ?", id, imageID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryImageNotFound
		}
		return nil, err
	}

	content, contentType, err := s.downloadStoredObject(galleryStoredObject{
		ObjectKey:  row.GCPObjectKey,
		StorageURL: row.FileURL,
	})
	if err != nil {
		return nil, err
	}

	return &GalleryMediaContent{
		Content:     content,
		ContentType: contentType,
		FileName: buildGalleryContentFileName(
			row.Title,
			row.GCPObjectKey,
			row.FileURL,
			contentType,
			"gallery-image",
		),
	}, nil
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

func (s *GalleryService) AddGalleryImages(id int, req AddGalleryImagesRequest, userID *int) (*AddGalleryImagesResponse, error) {
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

	startSortOrder, err := s.nextGalleryImageSortOrder(tx, id)
	if err != nil {
		tx.Rollback()
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
			Title:        input.Title,
			AltText:      input.AltText,
			GCPObjectKey: objectKey,
			FileURL:      fileURL,
			MimeType:     input.MimeType,
			FileSize:     fileSize,
			SortOrder:    startSortOrder + idx,
			UploadedBy:   userID,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := s.touchGalleryUpdatedAt(tx, id); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &AddGalleryImagesResponse{UploadedCount: len(req.Images)}, nil
}

func (s *GalleryService) UpdateGalleryImage(id int, imageID int, req UpdateGalleryImageRequest) (*GalleryAssetResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeUpdateGalleryImageRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	var row GalleryImage
	if err := tx.Where("gallery_id = ? AND id = ?", id, imageID).First(&row).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGalleryImageNotFound
		}
		return nil, err
	}

	row.Title = req.Title
	row.AltText = req.AltText

	if err := tx.Save(&row).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := s.touchGalleryUpdatedAt(tx, id); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	resp := mapGalleryAssetResponse(row)
	return &resp, nil
}

func (s *GalleryService) ReorderGalleryImages(id int, imageIDs []int) (*ReorderGalleryImagesResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	imageIDs, err := normalizeGalleryImageOrder(imageIDs)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	var rows []GalleryImage
	if err := tx.
		Where("gallery_id = ?", id).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if len(rows) == 0 {
		tx.Rollback()
		return nil, ErrGalleryImageNotFound
	}
	if len(rows) != len(imageIDs) {
		tx.Rollback()
		return nil, errors.New("image_ids must include every gallery image exactly once")
	}

	available := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		available[row.ID] = struct{}{}
	}
	for _, imageID := range imageIDs {
		if _, ok := available[imageID]; !ok {
			tx.Rollback()
			return nil, errors.New("image_ids must include every gallery image exactly once")
		}
	}

	for idx, imageID := range imageIDs {
		if err := tx.Model(&GalleryImage{}).
			Where("gallery_id = ? AND id = ?", id, imageID).
			Update("sort_order", idx).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := s.touchGalleryUpdatedAt(tx, id); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &ReorderGalleryImagesResponse{UpdatedCount: len(imageIDs)}, nil
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
	if err := s.resequenceGalleryImages(tx, id); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.touchGalleryUpdatedAt(tx, id); err != nil {
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

func normalizeUpdateGalleryImageRequest(req UpdateGalleryImageRequest) (UpdateGalleryImageRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.AltText = strings.TrimSpace(req.AltText)
	if req.Title == "" {
		return req, errors.New("title is required")
	}
	return req, nil
}

func normalizeGalleryImageOrder(imageIDs []int) ([]int, error) {
	if len(imageIDs) == 0 {
		return nil, errors.New("image_ids are required")
	}

	cleaned := make([]int, 0, len(imageIDs))
	seen := make(map[int]struct{}, len(imageIDs))
	for _, imageID := range imageIDs {
		if imageID <= 0 {
			return nil, errors.New("image_ids must be positive integers")
		}
		if _, exists := seen[imageID]; exists {
			return nil, errors.New("image_ids must not contain duplicates")
		}
		seen[imageID] = struct{}{}
		cleaned = append(cleaned, imageID)
	}

	return cleaned, nil
}

func sanitizeGalleryUploadInput(value GalleryUploadInput) GalleryUploadInput {
	value.Title = strings.TrimSpace(value.Title)
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

func mapGalleryAssetResponse(row GalleryImage) GalleryAssetResponse {
	return GalleryAssetResponse{
		ID:           row.ID,
		GalleryID:    row.GalleryID,
		Title:        row.Title,
		AltText:      row.AltText,
		FileName:     buildGalleryContentFileName(row.Title, row.GCPObjectKey, row.FileURL, row.MimeType, "gallery-image"),
		GCPObjectKey: row.GCPObjectKey,
		FileURL:      buildGalleryImageFetchURL(row.GalleryID, row.ID),
		StorageURI:   row.FileURL,
		MimeType:     row.MimeType,
		FileSize:     row.FileSize,
		SortOrder:    row.SortOrder,
		UploadedBy:   row.UploadedBy,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func mapGalleryCoverResponse(row Gallery) *GalleryAssetResponse {
	if strings.TrimSpace(row.CoverImageObjectKey) == "" && strings.TrimSpace(row.CoverImageURL) == "" {
		return nil
	}

	return &GalleryAssetResponse{
		ID:           0,
		GalleryID:    row.ID,
		Title:        row.CoverImageAltText,
		AltText:      row.CoverImageAltText,
		FileName:     buildGalleryContentFileName(row.CoverImageAltText, row.CoverImageObjectKey, row.CoverImageURL, "", "gallery-cover"),
		GCPObjectKey: row.CoverImageObjectKey,
		FileURL:      buildGalleryCoverFetchURL(row.ID),
		StorageURI:   row.CoverImageURL,
		MimeType:     "",
		FileSize:     0,
		SortOrder:    0,
		CreatedAt:    row.UpdatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
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

func (s *GalleryService) downloadStoredObject(item galleryStoredObject) ([]byte, string, error) {
	bucketName, objectName, err := s.resolveObjectReference(item)
	if err != nil {
		return nil, "", err
	}
	content, contentType, err := downloadGCSObjectHook(bucketName, objectName)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, "", ErrGalleryImageNotFound
		}
		return nil, "", err
	}
	return content, contentType, nil
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

func (s *GalleryService) nextGalleryImageSortOrder(tx *gorm.DB, galleryID int) (int, error) {
	type maxSortOrderRow struct {
		MaxSortOrder int `gorm:"column:max_sort_order"`
	}

	var row maxSortOrderRow
	if err := tx.Model(&GalleryImage{}).
		Select("COALESCE(MAX(sort_order), -1) AS max_sort_order").
		Where("gallery_id = ?", galleryID).
		Scan(&row).Error; err != nil {
		return 0, err
	}

	return row.MaxSortOrder + 1, nil
}

func (s *GalleryService) resequenceGalleryImages(tx *gorm.DB, galleryID int) error {
	var rows []GalleryImage
	if err := tx.
		Where("gallery_id = ?", galleryID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return err
	}

	for idx, row := range rows {
		if row.SortOrder == idx {
			continue
		}
		if err := tx.Model(&GalleryImage{}).
			Where("gallery_id = ? AND id = ?", galleryID, row.ID).
			Update("sort_order", idx).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *GalleryService) touchGalleryUpdatedAt(tx *gorm.DB, galleryID int) error {
	return tx.Model(&Gallery{}).
		Where("id = ?", galleryID).
		Update("updated_at", nowFunc()).Error
}

func buildGalleryCoverFetchURL(galleryID int) string {
	return fmt.Sprintf("/api/galleries/%d/cover/content", galleryID)
}

func buildGalleryImageFetchURL(galleryID int, imageID int) string {
	return fmt.Sprintf("/api/galleries/%d/images/%d/content", galleryID, imageID)
}

func buildGalleryContentFileName(title string, objectKey string, storageURL string, mimeType string, fallbackBase string) string {
	ext := path.Ext(strings.TrimSpace(objectKey))
	if ext == "" && strings.TrimSpace(storageURL) != "" {
		if _, parsedObjectKey, err := util.ParseGCSObjectReference("", storageURL); err == nil {
			ext = path.Ext(parsedObjectKey)
		}
	}
	if ext == "" {
		ext = util.ExtFromFilenameOrMime("", mimeType)
	}

	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle != "" {
		base := util.SanitizePart(trimmedTitle)
		if base != "unknown" {
			return base + ext
		}
	}

	if strings.TrimSpace(objectKey) != "" {
		baseName := path.Base(strings.TrimSpace(objectKey))
		if baseName != "." && baseName != "/" && baseName != "" {
			return baseName
		}
	}

	if strings.TrimSpace(storageURL) != "" {
		if _, parsedObjectKey, err := util.ParseGCSObjectReference("", storageURL); err == nil {
			baseName := path.Base(parsedObjectKey)
			if baseName != "." && baseName != "/" && baseName != "" {
				return baseName
			}
		}
	}

	if strings.TrimSpace(fallbackBase) == "" {
		fallbackBase = "gallery-asset"
	}
	return fallbackBase + ext
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

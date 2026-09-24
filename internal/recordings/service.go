package recordings

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
	ErrStoreUnavailable            = errors.New("recording store unavailable")
	ErrRecordingCollectionNotFound = errors.New("recording collection not found")
	ErrRecordingItemNotFound       = errors.New("recording item not found")
	ErrMediaBucketNotConfigured    = errors.New("drive bucket is not configured")
)

var (
	recordingNowFunc = time.Now

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

type RecordingService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

type recordingStoredObject struct {
	ObjectKey  string
	StorageURL string
}

func (s *RecordingService) ListRecordingCollections() (*RecordingCollectionListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var rows []RecordingCollection
	if err := s.DB.Order("name ASC").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	counts := map[int]int{}
	if len(rows) > 0 {
		type countRow struct {
			RecordingCollectionID int `gorm:"column:recording_collection_id"`
			Total                 int `gorm:"column:total"`
		}

		var countRows []countRow
		if err := s.DB.Model(&RecordingCollectionItem{}).
			Select("recording_collection_id, COUNT(*) AS total").
			Group("recording_collection_id").
			Scan(&countRows).Error; err != nil {
			return nil, err
		}
		for _, count := range countRows {
			counts[count.RecordingCollectionID] = count.Total
		}
	}

	items := make([]RecordingCollectionSummaryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, RecordingCollectionSummaryItem{
			ID:           row.ID,
			Name:         row.Name,
			PlacementKey: row.PlacementKey,
			ItemCount:    counts[row.ID],
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		})
	}

	return &RecordingCollectionListResponse{Items: items}, nil
}

func (s *RecordingService) GetRecordingCollection(id int) (*RecordingCollectionDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	row, err := s.getRecordingCollection(id)
	if err != nil {
		return nil, err
	}

	return s.buildRecordingCollectionDetail(row)
}

func (s *RecordingService) GetRecordingCollectionByTitle(title string) (*RecordingCollectionDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("title is required")
	}

	var row RecordingCollection
	if err := s.DB.Where("name = ?", title).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingCollectionNotFound
		}
		return nil, err
	}

	return s.buildRecordingCollectionDetail(row)
}

func (s *RecordingService) GetRecordingCollectionByPlacementKey(placementKey string) (*RecordingCollectionDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	placementKey = strings.TrimSpace(placementKey)
	if placementKey == "" {
		return nil, errors.New("placement key is required")
	}

	var row RecordingCollection
	if err := s.DB.Where("placement_key = ?", placementKey).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingCollectionNotFound
		}
		return nil, err
	}

	return s.buildRecordingCollectionDetail(row)
}

func (s *RecordingService) buildRecordingCollectionDetail(row RecordingCollection) (*RecordingCollectionDetailResponse, error) {
	items, err := s.loadRecordingItems(s.DB, row.ID)
	if err != nil {
		return nil, err
	}

	return &RecordingCollectionDetailResponse{
		ID:           row.ID,
		Name:         row.Name,
		PlacementKey: row.PlacementKey,
		ItemCount:    len(items),
		Items:        items,
		CreatedBy:    row.CreatedBy,
		UpdatedBy:    row.UpdatedBy,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func (s *RecordingService) GetRecordingItemContent(id int, itemID int) (*RecordingMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	if _, err := s.getRecordingCollection(id); err != nil {
		return nil, err
	}

	var row RecordingCollectionItem
	if err := s.DB.Where("recording_collection_id = ? AND id = ?", id, itemID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingItemNotFound
		}
		return nil, err
	}

	content, contentType, err := s.downloadStoredObject(recordingStoredObject{
		ObjectKey:  row.RecordingObjectKey,
		StorageURL: row.RecordingURL,
	})
	if err != nil {
		return nil, err
	}

	return &RecordingMediaContent{
		Content:     content,
		ContentType: contentType,
		FileName:    buildRecordingContentFileName(row.Title, row.RecordingObjectKey, row.RecordingURL, ""),
	}, nil
}

func (s *RecordingService) CreateRecordingCollection(req SaveRecordingCollectionRequest, userID *int) (*RecordingCollectionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeSaveRecordingCollectionRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	collection := RecordingCollection{
		Name:      req.Name,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := tx.Create(&collection).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	uploadedObjects := make([]string, 0, len(req.Items))
	if len(req.Items) > 0 {
		if err := s.createRecordingItems(tx, collection.ID, req.Items, 0, userID, &uploadedObjects); err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &RecordingCollectionMutationResponse{ID: collection.ID, Name: collection.Name}, nil
}

func (s *RecordingService) UpdateRecordingCollection(id int, req UpdateRecordingCollectionRequest, userID *int) (*RecordingCollectionMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	var row RecordingCollection
	if err := tx.First(&row, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingCollectionNotFound
		}
		return nil, err
	}

	row.Name = name
	row.UpdatedBy = userID
	if err := tx.Save(&row).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &RecordingCollectionMutationResponse{ID: row.ID, Name: row.Name}, nil
}

func (s *RecordingService) DeleteRecordingCollection(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackOnPanic(tx)

	var collection RecordingCollection
	if err := tx.First(&collection, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRecordingCollectionNotFound
		}
		return err
	}

	items, err := s.loadRecordingItemRows(tx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("recording_collection_id = ?", id).Delete(&RecordingCollectionItem{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&collection).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	objects := make([]recordingStoredObject, 0, len(items))
	for _, item := range items {
		objects = append(objects, recordingStoredObject{
			ObjectKey:  item.RecordingObjectKey,
			StorageURL: item.RecordingURL,
		})
	}

	return s.cleanupStoredObjects(objects)
}

func (s *RecordingService) AddRecordingItems(id int, req AddRecordingItemsRequest, userID *int) (*AddRecordingItemsResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeAddRecordingItemsRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	if _, err := s.getRecordingCollectionTx(tx, id); err != nil {
		tx.Rollback()
		return nil, err
	}

	nextSort, err := s.nextRecordingItemSortOrder(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	uploadedObjects := make([]string, 0, len(req.Items))
	if err := s.createRecordingItems(tx, id, req.Items, nextSort, userID, &uploadedObjects); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.touchRecordingCollection(tx, id, userID); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &AddRecordingItemsResponse{UploadedCount: len(req.Items)}, nil
}

func (s *RecordingService) UpdateRecordingItem(id int, itemID int, req UpdateRecordingItemRequest, userID *int) (*RecordingItemResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	if _, err := s.getRecordingCollectionTx(tx, id); err != nil {
		tx.Rollback()
		return nil, err
	}

	var row RecordingCollectionItem
	if err := tx.Where("recording_collection_id = ? AND id = ?", id, itemID).First(&row).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingItemNotFound
		}
		return nil, err
	}

	req = sanitizeRecordingItemInput(req)
	if strings.TrimSpace(req.Title) != "" {
		row.Title = req.Title
	}
	row.Description = req.Description

	if strings.TrimSpace(row.Title) == "" {
		tx.Rollback()
		return nil, errors.New("title is required")
	}

	oldObjects := make([]recordingStoredObject, 0, 1)
	uploadedObjects := make([]string, 0, 1)
	hasNewRecording := recordingInputHasMedia(req)
	if hasNewRecording {
		if err := validateRecordingMediaType(req); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if req.RemoveRecording && !hasNewRecording {
		oldObjects = append(oldObjects, recordingStoredObject{
			ObjectKey:  row.RecordingObjectKey,
			StorageURL: row.RecordingURL,
		})
		row.RecordingURL = ""
		row.RecordingObjectKey = ""
	} else if hasNewRecording {
		oldObjects = append(oldObjects, recordingStoredObject{
			ObjectKey:  row.RecordingObjectKey,
			StorageURL: row.RecordingURL,
		})

		recordingURL, objectKey, uploadedObject, err := s.storeRecording(id, row.SortOrder, req)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}

		row.RecordingURL = recordingURL
		row.RecordingObjectKey = objectKey
	}
	if strings.TrimSpace(row.RecordingURL) == "" && strings.TrimSpace(row.RecordingObjectKey) == "" {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, errors.New("recording file is required")
	}

	row.UpdatedBy = userID
	if err := tx.Save(&row).Error; err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := s.touchRecordingCollection(tx, id, userID); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if cleanupErr := s.cleanupStoredObjects(oldObjects); cleanupErr != nil {
		return nil, cleanupErr
	}

	resp := mapRecordingItemResponse(row)
	return &resp, nil
}

func (s *RecordingService) DeleteRecordingItem(id int, itemID int) (*DeleteRecordingItemResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	if _, err := s.getRecordingCollectionTx(tx, id); err != nil {
		tx.Rollback()
		return nil, err
	}

	var row RecordingCollectionItem
	if err := tx.Where("recording_collection_id = ? AND id = ?", id, itemID).First(&row).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingItemNotFound
		}
		return nil, err
	}

	if err := tx.Delete(&row).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := s.touchRecordingCollection(tx, id, nil); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	if cleanupErr := s.cleanupStoredObjects([]recordingStoredObject{{
		ObjectKey:  row.RecordingObjectKey,
		StorageURL: row.RecordingURL,
	}}); cleanupErr != nil {
		return nil, cleanupErr
	}

	return &DeleteRecordingItemResponse{DeletedCount: 1}, nil
}

func normalizeSaveRecordingCollectionRequest(req SaveRecordingCollectionRequest) (SaveRecordingCollectionRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return req, errors.New("name is required")
	}

	items := make([]RecordingItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		item = sanitizeRecordingItemInput(item)
		if err := validateRecordingItemInput(item); err != nil {
			return req, err
		}
		items = append(items, item)
	}
	req.Items = items
	return req, nil
}

func normalizeAddRecordingItemsRequest(req AddRecordingItemsRequest) (AddRecordingItemsRequest, error) {
	items := make([]RecordingItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		item = sanitizeRecordingItemInput(item)
		if err := validateRecordingItemInput(item); err != nil {
			return req, err
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return req, errors.New("items are required")
	}
	req.Items = items
	return req, nil
}

func sanitizeRecordingItemInput(input RecordingItemInput) RecordingItemInput {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.FileName = strings.TrimSpace(input.FileName)
	input.MimeType = strings.TrimSpace(input.MimeType)
	input.FileURL = strings.TrimSpace(input.FileURL)
	input.StorageURI = strings.TrimSpace(input.StorageURI)
	input.RecordingURL = strings.TrimSpace(input.RecordingURL)
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.GCPObjectKey = strings.TrimSpace(input.GCPObjectKey)
	return input
}

func validateRecordingItemInput(input RecordingItemInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return errors.New("title is required")
	}
	if !recordingInputHasMedia(input) {
		return errors.New("recording file is required")
	}
	if err := validateRecordingMediaType(input); err != nil {
		return err
	}
	return nil
}

func validateRecordingMediaType(input RecordingItemInput) error {
	if !recordingInputHasMedia(input) {
		return nil
	}

	mimeType := strings.ToLower(strings.TrimSpace(input.MimeType))
	if mimeType != "" && !strings.HasPrefix(mimeType, "audio/") {
		return errors.New("only audio recording uploads are supported")
	}

	extension := strings.ToLower(path.Ext(strings.TrimSpace(input.FileName)))
	if mimeType == "" && (len(input.Content) > 0 || strings.TrimSpace(input.DataBase64) != "") {
		switch extension {
		case ".aac", ".flac", ".m4a", ".mp3", ".oga", ".ogg", ".wav", ".webm":
		default:
			return errors.New("only audio recording uploads are supported")
		}
	}

	return nil
}

func recordingInputHasMedia(input RecordingItemInput) bool {
	return len(input.Content) > 0 ||
		strings.TrimSpace(input.DataBase64) != "" ||
		strings.TrimSpace(input.FileURL) != "" ||
		strings.TrimSpace(input.StorageURI) != "" ||
		strings.TrimSpace(input.RecordingURL) != "" ||
		strings.TrimSpace(input.ObjectKey) != "" ||
		strings.TrimSpace(input.GCPObjectKey) != ""
}

func mapRecordingItemResponse(row RecordingCollectionItem) RecordingItemResponse {
	recordingURL := ""
	if strings.TrimSpace(row.RecordingURL) != "" || strings.TrimSpace(row.RecordingObjectKey) != "" {
		recordingURL = buildRecordingItemFetchURL(row.RecordingCollectionID, row.ID)
	}

	return RecordingItemResponse{
		ID:                    row.ID,
		RecordingCollectionID: row.RecordingCollectionID,
		Title:                 row.Title,
		Description:           row.Description,
		RecordingURL:          recordingURL,
		SortOrder:             row.SortOrder,
		CreatedBy:             row.CreatedBy,
		UpdatedBy:             row.UpdatedBy,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func (s *RecordingService) getRecordingCollection(id int) (RecordingCollection, error) {
	return s.getRecordingCollectionTx(s.DB, id)
}

func (s *RecordingService) getRecordingCollectionTx(db *gorm.DB, id int) (RecordingCollection, error) {
	var row RecordingCollection
	if err := db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RecordingCollection{}, ErrRecordingCollectionNotFound
		}
		return RecordingCollection{}, err
	}
	return row, nil
}

func (s *RecordingService) loadRecordingItemRows(db *gorm.DB, id int) ([]RecordingCollectionItem, error) {
	var rows []RecordingCollectionItem
	if err := db.
		Where("recording_collection_id = ?", id).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *RecordingService) loadRecordingItems(db *gorm.DB, id int) ([]RecordingItemResponse, error) {
	rows, err := s.loadRecordingItemRows(db, id)
	if err != nil {
		return nil, err
	}

	items := make([]RecordingItemResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapRecordingItemResponse(row))
	}
	return items, nil
}

func (s *RecordingService) createRecordingItems(tx *gorm.DB, collectionID int, items []RecordingItemInput, startSort int, userID *int, uploadedObjects *[]string) error {
	for idx, item := range items {
		recordingURL, objectKey, uploadedObject, err := s.storeRecording(collectionID, startSort+idx, item)
		if err != nil {
			return err
		}
		if uploadedObject != "" && uploadedObjects != nil {
			*uploadedObjects = append(*uploadedObjects, uploadedObject)
		}

		row := RecordingCollectionItem{
			RecordingCollectionID: collectionID,
			Title:                 item.Title,
			Description:           item.Description,
			RecordingURL:          recordingURL,
			RecordingObjectKey:    objectKey,
			SortOrder:             startSort + idx,
			CreatedBy:             userID,
			UpdatedBy:             userID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *RecordingService) storeRecording(collectionID int, index int, input RecordingItemInput) (string, string, string, error) {
	referenceURL := strings.TrimSpace(input.StorageURI)
	if referenceURL == "" {
		referenceURL = strings.TrimSpace(input.FileURL)
	}
	if referenceURL == "" {
		referenceURL = strings.TrimSpace(input.RecordingURL)
	}

	objectKey := strings.TrimSpace(input.ObjectKey)
	if objectKey == "" {
		objectKey = strings.TrimSpace(input.GCPObjectKey)
	}

	if len(input.Content) == 0 && strings.TrimSpace(input.DataBase64) == "" {
		if referenceURL == "" {
			// A recording is optional: persist the item with no media attached.
			return "", "", "", nil
		}
		if objectKey == "" {
			_, resolvedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), referenceURL)
			if err == nil {
				objectKey = s.relativeObjectKey(resolvedObjectKey)
			}
		}
		return referenceURL, objectKey, "", nil
	}

	if strings.TrimSpace(s.BucketName) == "" {
		return "", "", "", ErrMediaBucketNotConfigured
	}

	objectName := s.recordingObjectName(collectionID, index, input.FileName, input.MimeType)
	storageObjectName := s.storageObjectName(objectName)

	var err error
	fileURL := ""
	if len(input.Content) > 0 {
		fileURL, _, err = uploadBytesToGCSHook(input.Content, s.BucketName, storageObjectName, input.MimeType)
	} else {
		fileURL, _, err = uploadBase64ToGCSHook(input.DataBase64, s.BucketName, storageObjectName, input.MimeType)
	}
	if err != nil {
		return "", "", "", err
	}

	return fileURL, objectName, storageObjectName, nil
}

func (s *RecordingService) recordingObjectName(collectionID int, index int, fileName string, mimeType string) string {
	timestamp := recordingNowFunc().UTC().Format("20060102150405")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = "recording"
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("recordings/%d/items/%s_%d_%s%s", collectionID, timestamp, index+1, base, ext)
}

func (s *RecordingService) nextRecordingItemSortOrder(tx *gorm.DB, collectionID int) (int, error) {
	type maxSortOrderRow struct {
		MaxSortOrder int `gorm:"column:max_sort_order"`
	}

	var row maxSortOrderRow
	if err := tx.Model(&RecordingCollectionItem{}).
		Select("COALESCE(MAX(sort_order), -1) AS max_sort_order").
		Where("recording_collection_id = ?", collectionID).
		Scan(&row).Error; err != nil {
		return 0, err
	}
	return row.MaxSortOrder + 1, nil
}

func (s *RecordingService) touchRecordingCollection(tx *gorm.DB, id int, userID *int) error {
	updates := map[string]any{
		"updated_at": recordingNowFunc(),
	}
	if userID != nil {
		updates["updated_by"] = userID
	}
	return tx.Model(&RecordingCollection{}).Where("id = ?", id).Updates(updates).Error
}

func (s *RecordingService) cleanupStoredObjects(objects []recordingStoredObject) error {
	var cleanupErr error
	for _, object := range objects {
		if err := s.cleanupStoredObject(object); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func (s *RecordingService) cleanupStoredObject(object recordingStoredObject) error {
	bucketName, objectName, err := s.resolveObjectReference(object)
	if err != nil {
		return err
	}
	if objectName == "" {
		return nil
	}
	return deleteGCSObjectHook(bucketName, objectName)
}

func (s *RecordingService) resolveObjectReference(object recordingStoredObject) (string, string, error) {
	objectKey := strings.TrimSpace(object.ObjectKey)
	if objectKey != "" {
		bucketName := strings.TrimSpace(s.BucketName)
		if bucketName == "" {
			return "", "", ErrMediaBucketNotConfigured
		}
		return bucketName, s.storageObjectName(objectKey), nil
	}
	if strings.TrimSpace(object.StorageURL) == "" {
		return "", "", nil
	}

	bucketName, objectName, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), object.StorageURL)
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

func (s *RecordingService) downloadStoredObject(object recordingStoredObject) ([]byte, string, error) {
	bucketName, objectName, err := s.resolveObjectReference(object)
	if err != nil {
		return nil, "", err
	}
	if objectName == "" {
		return nil, "", ErrRecordingItemNotFound
	}

	content, contentType, err := downloadGCSObjectHook(bucketName, objectName)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, "", ErrRecordingItemNotFound
		}
		return nil, "", err
	}
	return content, contentType, nil
}

func (s *RecordingService) cleanupObjects(objectNames []string) {
	for _, objectName := range objectNames {
		if strings.TrimSpace(objectName) == "" || strings.TrimSpace(s.BucketName) == "" {
			continue
		}
		_ = deleteGCSObjectHook(s.BucketName, objectName)
	}
}

func (s *RecordingService) storageObjectName(objectKey string) string {
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

func (s *RecordingService) relativeObjectKey(objectKey string) string {
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

func buildRecordingItemFetchURL(collectionID int, itemID int) string {
	return fmt.Sprintf("/api/recordings/%d/items/%d/content", collectionID, itemID)
}

func buildRecordingContentFileName(title string, objectKey string, storageURL string, mimeType string) string {
	ext := path.Ext(strings.TrimSpace(objectKey))
	if ext == "" && strings.TrimSpace(storageURL) != "" {
		if _, parsedObjectKey, err := util.ParseGCSObjectReference("", storageURL); err == nil {
			ext = path.Ext(parsedObjectKey)
		}
	}
	if ext == "" {
		ext = util.ExtFromFilenameOrMime("", mimeType)
	}

	if trimmedTitle := strings.TrimSpace(title); trimmedTitle != "" {
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

	return "recording" + ext
}

func rollbackOnPanic(tx *gorm.DB) {
	if recover() != nil {
		tx.Rollback()
		panic("transaction panic")
	}
}

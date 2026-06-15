package video

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
	ErrStoreUnavailable         = errors.New("video store unavailable")
	ErrVideoPackageNotFound     = errors.New("video package not found")
	ErrVideoItemNotFound        = errors.New("video item not found")
	ErrCollectionPackageOnly    = errors.New("collection packages only support add and delete item operations")
	ErrMediaBucketNotConfigured = errors.New("drive bucket is not configured")
)

var (
	videoNowFunc          = time.Now
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

type VideoService struct {
	DB           *gorm.DB
	BucketName   string
	BucketPrefix string
}

type videoStoredObject struct {
	ObjectKey  string
	StorageURL string
}

func (s *VideoService) ListVideoPackages() (*VideoPackageListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var rows []VideoPackage
	if err := s.DB.Order("LOWER(title) ASC").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]VideoPackageSummaryItem, 0, len(rows))
	if len(rows) == 0 {
		return &VideoPackageListResponse{Items: items}, nil
	}

	packageIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		packageIDs = append(packageIDs, row.ID)
	}

	type countRow struct {
		VideoPackageID int
		Count          int64
	}
	var countRows []countRow
	if err := s.DB.
		Model(&VideoItem{}).
		Select("video_package_id, COUNT(*) AS count").
		Where("video_package_id IN ?", packageIDs).
		Group("video_package_id").
		Scan(&countRows).Error; err != nil {
		return nil, err
	}

	countsByPackage := make(map[int]int, len(countRows))
	for _, row := range countRows {
		countsByPackage[row.VideoPackageID] = int(row.Count)
	}

	var itemRows []VideoItem
	if err := s.DB.
		Where("video_package_id IN ?", packageIDs).
		Order("video_package_id ASC").
		Order("sort_order ASC").
		Order("id ASC").
		Find(&itemRows).Error; err != nil {
		return nil, err
	}

	firstItemByPackage := make(map[int]VideoItem, len(itemRows))
	for _, row := range itemRows {
		if _, exists := firstItemByPackage[row.VideoPackageID]; !exists {
			firstItemByPackage[row.VideoPackageID] = row
		}
	}

	for _, row := range rows {
		frontImageURL := ""
		if item, ok := firstItemByPackage[row.ID]; ok &&
			(strings.TrimSpace(item.TeaserImageURL) != "" || strings.TrimSpace(item.TeaserImageObjectKey) != "") {
			frontImageURL = buildVideoTeaserFetchURL(row.ID, item.ID)
		}

		items = append(items, VideoPackageSummaryItem{
			ID:            row.ID,
			Title:         row.Title,
			PackageType:   row.PackageType,
			VideoCount:    countsByPackage[row.ID],
			FrontImageURL: frontImageURL,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		})
	}

	return &VideoPackageListResponse{Items: items}, nil
}

func (s *VideoService) GetVideoPackage(id int) (*VideoPackageDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	row, err := s.getVideoPackage(id)
	if err != nil {
		return nil, err
	}

	items, err := s.loadVideoItems(s.DB, id)
	if err != nil {
		return nil, err
	}

	responses := make([]VideoItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, mapVideoItemResponse(item))
	}

	var singleVideo *VideoItemResponse
	if row.PackageType == VideoPackageTypeSingle && len(responses) > 0 {
		copyItem := responses[0]
		singleVideo = &copyItem
	}

	return &VideoPackageDetailResponse{
		ID:          row.ID,
		Title:       row.Title,
		PackageType: row.PackageType,
		VideoCount:  len(responses),
		SingleVideo: singleVideo,
		Videos:      responses,
		CreatedBy:   row.CreatedBy,
		UpdatedBy:   row.UpdatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

func (s *VideoService) GetVideoTeaserContent(id int, itemID int) (*VideoMediaContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var row VideoItem
	if err := s.DB.Where("video_package_id = ? AND id = ?", id, itemID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVideoItemNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(row.TeaserImageURL) == "" && strings.TrimSpace(row.TeaserImageObjectKey) == "" {
		return nil, ErrVideoItemNotFound
	}

	content, contentType, err := s.downloadStoredObject(videoStoredObject{
		ObjectKey:  row.TeaserImageObjectKey,
		StorageURL: row.TeaserImageURL,
	})
	if err != nil {
		return nil, err
	}

	return &VideoMediaContent{
		Content:     content,
		ContentType: contentType,
		FileName:    buildVideoContentFileName(row.Title, row.TeaserImageObjectKey, row.TeaserImageURL, contentType, "video-teaser"),
	}, nil
}

func (s *VideoService) CreateVideoPackage(req SaveVideoPackageRequest, userID *int) (*VideoPackageMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, items, err := normalizeCreateVideoPackageRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	uploadedObjects := make([]string, 0, len(items))

	row := VideoPackage{
		Title:       req.Title,
		PackageType: req.PackageType,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}
	if err := tx.Create(&row).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := s.createVideoItems(tx, row.ID, items, userID, &uploadedObjects); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &VideoPackageMutationResponse{ID: row.ID, Title: row.Title, PackageType: row.PackageType}, nil
}

func (s *VideoService) UpdateVideoPackage(id int, req UpdateVideoPackageRequest, userID *int) (*VideoPackageMutationResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	row, err := s.getVideoPackageTx(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	row.Title = req.Title
	row.UpdatedBy = userID
	if err := tx.Save(&row).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if row.PackageType == VideoPackageTypeSingle {
		var items []VideoItem
		if err := tx.
			Where("video_package_id = ?", id).
			Order("sort_order ASC").
			Order("id ASC").
			Find(&items).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if len(items) > 0 {
			items[0].Title = req.Title
			items[0].UpdatedBy = userID
			if err := tx.Save(&items[0]).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &VideoPackageMutationResponse{ID: row.ID, Title: row.Title, PackageType: row.PackageType}, nil
}

func (s *VideoService) DeleteVideoPackage(id int) error {
	if s.DB == nil {
		return ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer rollbackOnPanic(tx)

	row, err := s.getVideoPackageTx(tx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	items, err := s.loadVideoItems(tx, id)
	if err != nil {
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

	cleanupObjects := make([]videoStoredObject, 0, len(items))
	for _, item := range items {
		cleanupObjects = append(cleanupObjects, videoStoredObject{
			ObjectKey:  item.TeaserImageObjectKey,
			StorageURL: item.TeaserImageURL,
		})
	}
	return s.cleanupStoredObjects(cleanupObjects)
}

func (s *VideoService) AddVideoItems(id int, req AddVideoItemsRequest, userID *int) (*AddVideoItemsResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	req, err := normalizeAddVideoItemsRequest(req)
	if err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	row, err := s.getVideoPackageTx(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if row.PackageType != VideoPackageTypeCollection {
		tx.Rollback()
		return nil, ErrCollectionPackageOnly
	}

	nextSortOrder, err := s.nextVideoItemSortOrder(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	uploadedObjects := make([]string, 0, len(req.Videos))
	for idx, input := range req.Videos {
		teaserURL, objectKey, uploadedObject, err := s.storeVideoTeaser(id, nextSortOrder+idx, input)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}

		item := VideoItem{
			VideoPackageID:       id,
			Title:                input.Title,
			YouTubeURL:           input.YouTubeURL,
			Description:          input.Description,
			TeaserImageURL:       teaserURL,
			TeaserImageObjectKey: objectKey,
			SortOrder:            nextSortOrder + idx,
			CreatedBy:            userID,
			UpdatedBy:            userID,
		}
		if err := tx.Create(&item).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	}

	if err := s.touchVideoPackage(tx, id, userID); err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	return &AddVideoItemsResponse{UploadedCount: len(req.Videos)}, nil
}

func (s *VideoService) UpdateVideoItem(id int, itemID int, req UpdateVideoItemRequest, userID *int) (*VideoItemResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	pkg, err := s.getVideoPackageTx(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	var row VideoItem
	if err := tx.Where("video_package_id = ? AND id = ?", id, itemID).First(&row).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVideoItemNotFound
		}
		return nil, err
	}

	req = sanitizeVideoInput(req)
	if req.Title != "" {
		row.Title = req.Title
	}
	if req.YouTubeURL != "" {
		row.YouTubeURL = req.YouTubeURL
	}
	row.Description = req.Description

	if strings.TrimSpace(row.Title) == "" {
		tx.Rollback()
		return nil, errors.New("title is required")
	}
	if strings.TrimSpace(row.YouTubeURL) == "" {
		tx.Rollback()
		return nil, errors.New("youtube_url is required")
	}
	if err := validateYouTubeURL(row.YouTubeURL); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := validateVideoTeaserInput(req); err != nil {
		tx.Rollback()
		return nil, err
	}

	uploadedObjects := make([]string, 0, 1)
	oldObjects := make([]videoStoredObject, 0, 1)
	hasNewTeaser := len(req.Content) > 0 ||
		strings.TrimSpace(req.DataBase64) != "" ||
		strings.TrimSpace(req.StorageURI) != "" ||
		strings.TrimSpace(req.FileURL) != "" ||
		strings.TrimSpace(req.ObjectKey) != "" ||
		strings.TrimSpace(req.GCPObjectKey) != ""

	if req.RemoveTeaserImage && !hasNewTeaser {
		tx.Rollback()
		return nil, errors.New("teaser image is required")
	}

	if hasNewTeaser {
		oldObjects = append(oldObjects, videoStoredObject{
			ObjectKey:  row.TeaserImageObjectKey,
			StorageURL: row.TeaserImageURL,
		})
		teaserURL, objectKey, uploadedObject, err := s.storeVideoTeaser(id, row.SortOrder, req)
		if err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
		if uploadedObject != "" {
			uploadedObjects = append(uploadedObjects, uploadedObject)
		}
		row.TeaserImageURL = teaserURL
		row.TeaserImageObjectKey = objectKey
	}

	row.UpdatedBy = userID
	if err := tx.Save(&row).Error; err != nil {
		tx.Rollback()
		s.cleanupObjects(uploadedObjects)
		return nil, err
	}

	if pkg.PackageType == VideoPackageTypeSingle {
		pkg.Title = row.Title
		pkg.UpdatedBy = userID
		if err := tx.Save(&pkg).Error; err != nil {
			tx.Rollback()
			s.cleanupObjects(uploadedObjects)
			return nil, err
		}
	} else if err := s.touchVideoPackage(tx, id, userID); err != nil {
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

	resp := mapVideoItemResponse(row)
	return &resp, nil
}

func (s *VideoService) DeleteVideoItem(id int, itemID int) (*DeleteVideoItemResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	pkg, err := s.getVideoPackageTx(tx, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if pkg.PackageType != VideoPackageTypeCollection {
		tx.Rollback()
		return nil, ErrCollectionPackageOnly
	}

	var row VideoItem
	if err := tx.Where("video_package_id = ? AND id = ?", id, itemID).First(&row).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVideoItemNotFound
		}
		return nil, err
	}

	if err := tx.Delete(&row).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.resequenceVideoItems(tx, id); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := s.touchVideoPackage(tx, id, nil); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	if err := s.cleanupStoredObjects([]videoStoredObject{{
		ObjectKey:  row.TeaserImageObjectKey,
		StorageURL: row.TeaserImageURL,
	}}); err != nil {
		return nil, err
	}

	return &DeleteVideoItemResponse{DeletedCount: 1}, nil
}

func normalizeCreateVideoPackageRequest(req SaveVideoPackageRequest) (SaveVideoPackageRequest, []VideoInput, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.PackageType = strings.ToLower(strings.TrimSpace(req.PackageType))

	switch req.PackageType {
	case VideoPackageTypeSingle:
		if req.SingleVideo == nil {
			return req, nil, errors.New("single_video is required when package_type is single")
		}
		item := sanitizeVideoInput(*req.SingleVideo)
		if item.Title == "" && req.Title != "" {
			item.Title = req.Title
		}
		if req.Title == "" {
			req.Title = item.Title
		}
		if req.Title == "" {
			return req, nil, errors.New("title is required")
		}
		if item.Title == "" {
			item.Title = req.Title
		}
		if err := validateVideoInput(item, true); err != nil {
			return req, nil, err
		}
		req.SingleVideo = &item
		return req, []VideoInput{item}, nil
	case VideoPackageTypeCollection:
		if req.Title == "" {
			return req, nil, errors.New("title is required")
		}
		items := make([]VideoInput, 0, len(req.Videos))
		for _, item := range req.Videos {
			item = sanitizeVideoInput(item)
			if err := validateVideoInput(item, true); err != nil {
				return req, nil, err
			}
			items = append(items, item)
		}
		req.Videos = items
		return req, items, nil
	default:
		return req, nil, errors.New("package_type must be one of single, collection")
	}
}

func normalizeAddVideoItemsRequest(req AddVideoItemsRequest) (AddVideoItemsRequest, error) {
	if len(req.Videos) == 0 {
		return req, errors.New("videos are required")
	}

	items := make([]VideoInput, 0, len(req.Videos))
	for _, item := range req.Videos {
		item = sanitizeVideoInput(item)
		if err := validateVideoInput(item, true); err != nil {
			return req, err
		}
		items = append(items, item)
	}
	req.Videos = items
	return req, nil
}

func sanitizeVideoInput(value VideoInput) VideoInput {
	value.Title = strings.TrimSpace(value.Title)
	value.YouTubeURL = strings.TrimSpace(value.YouTubeURL)
	value.Description = strings.TrimSpace(value.Description)
	value.FileName = strings.TrimSpace(value.FileName)
	value.MimeType = strings.TrimSpace(value.MimeType)
	value.DataBase64 = strings.TrimSpace(value.DataBase64)
	value.FileURL = strings.TrimSpace(value.FileURL)
	value.StorageURI = strings.TrimSpace(value.StorageURI)
	value.ObjectKey = strings.TrimSpace(value.ObjectKey)
	value.GCPObjectKey = strings.TrimSpace(value.GCPObjectKey)
	return value
}

func validateVideoInput(value VideoInput, requireTeaser bool) error {
	if strings.TrimSpace(value.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(value.YouTubeURL) == "" {
		return errors.New("youtube_url is required")
	}
	if err := validateYouTubeURL(value.YouTubeURL); err != nil {
		return err
	}
	if err := validateVideoTeaserInput(value); err != nil {
		return err
	}

	if requireTeaser {
		if len(value.Content) == 0 &&
			strings.TrimSpace(value.DataBase64) == "" &&
			strings.TrimSpace(value.StorageURI) == "" &&
			strings.TrimSpace(value.FileURL) == "" {
			return errors.New("teaser image is missing both uploaded file and file_url")
		}
	}

	return nil
}

func validateVideoTeaserInput(value VideoInput) error {
	mimeType := strings.ToLower(strings.TrimSpace(value.MimeType))
	if mimeType != "" && !strings.HasPrefix(mimeType, "image/") {
		return errors.New("only image uploads are supported")
	}
	if mimeType == "" && strings.TrimSpace(value.DataBase64) != "" {
		ext := strings.ToLower(util.ExtFromFilenameOrMime(value.FileName, value.MimeType))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		default:
			return errors.New("only image uploads are supported")
		}
	}
	return nil
}

func validateYouTubeURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil {
		return errors.New("youtube_url must be a valid YouTube URL")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	host = strings.TrimPrefix(host, "www.")
	if host == "youtube.com" || host == "m.youtube.com" || host == "youtu.be" || host == "youtube-nocookie.com" {
		return nil
	}
	return errors.New("youtube_url must be a valid YouTube URL")
}

func mapVideoItemResponse(row VideoItem) VideoItemResponse {
	return VideoItemResponse{
		ID:             row.ID,
		VideoPackageID: row.VideoPackageID,
		Title:          row.Title,
		YouTubeURL:     row.YouTubeURL,
		Description:    row.Description,
		TeaserImageURL: buildVideoTeaserFetchURL(row.VideoPackageID, row.ID),
		StorageURI:     row.TeaserImageURL,
		GCPObjectKey:   row.TeaserImageObjectKey,
		SortOrder:      row.SortOrder,
		CreatedBy:      row.CreatedBy,
		UpdatedBy:      row.UpdatedBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func (s *VideoService) getVideoPackage(id int) (VideoPackage, error) {
	return s.getVideoPackageTx(s.DB, id)
}

func (s *VideoService) getVideoPackageTx(db *gorm.DB, id int) (VideoPackage, error) {
	var row VideoPackage
	if err := db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return VideoPackage{}, ErrVideoPackageNotFound
		}
		return VideoPackage{}, err
	}
	return row, nil
}

func (s *VideoService) loadVideoItems(db *gorm.DB, packageID int) ([]VideoItem, error) {
	var rows []VideoItem
	if err := db.
		Where("video_package_id = ?", packageID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *VideoService) createVideoItems(tx *gorm.DB, packageID int, items []VideoInput, userID *int, uploadedObjects *[]string) error {
	for idx, item := range items {
		teaserURL, objectKey, uploadedObject, err := s.storeVideoTeaser(packageID, idx, item)
		if err != nil {
			return err
		}
		if uploadedObject != "" && uploadedObjects != nil {
			*uploadedObjects = append(*uploadedObjects, uploadedObject)
		}

		row := VideoItem{
			VideoPackageID:       packageID,
			Title:                item.Title,
			YouTubeURL:           item.YouTubeURL,
			Description:          item.Description,
			TeaserImageURL:       teaserURL,
			TeaserImageObjectKey: objectKey,
			SortOrder:            idx,
			CreatedBy:            userID,
			UpdatedBy:            userID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *VideoService) storeVideoTeaser(packageID int, idx int, input VideoInput) (string, string, string, error) {
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
			return "", "", "", errors.New("teaser image is missing both uploaded file and file_url")
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

	objectName := s.videoTeaserObjectName(packageID, idx, input.FileName, input.MimeType)
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

func (s *VideoService) videoTeaserObjectName(packageID int, idx int, fileName string, mimeType string) string {
	timestamp := videoNowFunc().UTC().Format("20060102150405")
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = "teaser"
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("videos/%d/items/%s_%d_%s%s", packageID, timestamp, idx+1, base, ext)
}

func (s *VideoService) nextVideoItemSortOrder(tx *gorm.DB, packageID int) (int, error) {
	type maxSortOrderRow struct {
		MaxSortOrder int `gorm:"column:max_sort_order"`
	}

	var row maxSortOrderRow
	if err := tx.Model(&VideoItem{}).
		Select("COALESCE(MAX(sort_order), -1) AS max_sort_order").
		Where("video_package_id = ?", packageID).
		Scan(&row).Error; err != nil {
		return 0, err
	}
	return row.MaxSortOrder + 1, nil
}

func (s *VideoService) resequenceVideoItems(tx *gorm.DB, packageID int) error {
	rows, err := s.loadVideoItems(tx, packageID)
	if err != nil {
		return err
	}

	for idx, row := range rows {
		if row.SortOrder == idx {
			continue
		}
		if err := tx.Model(&VideoItem{}).
			Where("video_package_id = ? AND id = ?", packageID, row.ID).
			Update("sort_order", idx).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *VideoService) touchVideoPackage(tx *gorm.DB, packageID int, userID *int) error {
	updates := map[string]any{
		"updated_at": videoNowFunc(),
	}
	if userID != nil {
		updates["updated_by"] = userID
	}
	return tx.Model(&VideoPackage{}).Where("id = ?", packageID).Updates(updates).Error
}

func (s *VideoService) cleanupStoredObjects(items []videoStoredObject) error {
	var cleanupErr error
	for _, item := range items {
		if err := s.cleanupStoredObject(item); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func (s *VideoService) cleanupStoredObject(item videoStoredObject) error {
	bucketName, objectName, err := s.resolveObjectReference(item)
	if err != nil {
		return err
	}
	if objectName == "" {
		return nil
	}
	return deleteGCSObjectHook(bucketName, objectName)
}

func (s *VideoService) resolveObjectReference(item videoStoredObject) (string, string, error) {
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

func (s *VideoService) downloadStoredObject(item videoStoredObject) ([]byte, string, error) {
	bucketName, objectName, err := s.resolveObjectReference(item)
	if err != nil {
		return nil, "", err
	}
	content, contentType, err := downloadGCSObjectHook(bucketName, objectName)
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, "", ErrVideoItemNotFound
		}
		return nil, "", err
	}
	return content, contentType, nil
}

func (s *VideoService) cleanupObjects(objectNames []string) {
	for _, objectName := range objectNames {
		if strings.TrimSpace(objectName) == "" || strings.TrimSpace(s.BucketName) == "" {
			continue
		}
		_ = deleteGCSObjectHook(s.BucketName, objectName)
	}
}

func (s *VideoService) storageObjectName(objectKey string) string {
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

func (s *VideoService) relativeObjectKey(objectKey string) string {
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

func buildVideoTeaserFetchURL(packageID int, itemID int) string {
	return fmt.Sprintf("/api/videos/%d/items/%d/teaser/content", packageID, itemID)
}

func buildVideoContentFileName(title string, objectKey string, storageURL string, mimeType string, fallbackBase string) string {
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

	if strings.TrimSpace(storageURL) != "" {
		if _, parsedObjectKey, err := util.ParseGCSObjectReference("", storageURL); err == nil {
			baseName := path.Base(parsedObjectKey)
			if baseName != "." && baseName != "/" && baseName != "" {
				return baseName
			}
		}
	}

	if strings.TrimSpace(fallbackBase) == "" {
		fallbackBase = "video-teaser"
	}
	return fallbackBase + ext
}

func rollbackOnPanic(tx *gorm.DB) {
	if recover() != nil {
		tx.Rollback()
		panic("transaction panic")
	}
}

package blogs

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

func normalizeSaveBlogRequest(req SaveBlogRequest) (SaveBlogRequest, time.Time, error) {
	req.Heading = strings.TrimSpace(req.Heading)
	req.Description = strings.TrimSpace(req.Description)
	req.PublishDate = strings.TrimSpace(req.PublishDate)
	if req.CoverImage != nil {
		cleaned := sanitizeUploadInput(*req.CoverImage)
		if isEmptyBlogUploadInput(cleaned) {
			req.CoverImage = nil
		} else {
			req.CoverImage = &cleaned
		}
	}
	if req.BlogDetail == nil {
		req.BlogDetail = &SaveBlogDetailRequest{Sections: make([]SaveBlogSectionRequest, 0)}
	}
	normalizedDetail, err := normalizeSaveBlogDetailRequest(req.BlogDetail)
	if err != nil {
		return req, time.Time{}, err
	}
	req.BlogDetail = normalizedDetail
	if req.Heading == "" {
		return req, time.Time{}, errors.New("heading is required")
	}
	if req.Description == "" {
		return req, time.Time{}, errors.New("description is required")
	}
	if req.PublishDate == "" {
		return req, time.Time{}, errors.New("publish_date is required")
	}
	publishDate, err := time.Parse("2006-01-02", req.PublishDate)
	if err != nil {
		return req, time.Time{}, errors.New("publish_date must use YYYY-MM-DD format")
	}
	return req, publishDate, nil
}

func normalizeSaveBlogDetailRequest(input *SaveBlogDetailRequest) (*SaveBlogDetailRequest, error) {
	if input == nil {
		return nil, nil
	}
	next := *input
	settings, err := normalizeJSONObject(next.Settings, "blog_detail.settings")
	if err != nil {
		return nil, err
	}
	next.Settings = settings
	next.Sections = append([]SaveBlogSectionRequest(nil), next.Sections...)
	for index := range next.Sections {
		normalized, err := normalizeSaveBlogSectionRequest(next.Sections[index], index)
		if err != nil {
			return nil, err
		}
		next.Sections[index] = normalized
	}
	return &next, nil
}

func normalizeSaveBlogSectionRequest(input SaveBlogSectionRequest, index int) (SaveBlogSectionRequest, error) {
	if input.ID != nil && *input.ID <= 0 {
		return input, fmt.Errorf("blog_detail.sections[%d].id must be a positive integer", index)
	}
	input.SectionName = strings.TrimSpace(input.SectionName)
	input.SectionType = strings.ToLower(strings.TrimSpace(input.SectionType))
	input.SortOrder = index
	if !isAllowed(input.SectionType,
		BlogSectionTypeHeading,
		BlogSectionTypeImage,
		BlogSectionTypeTypography,
		BlogSectionTypeAction,
		BlogSectionTypeVideo,
		BlogSectionTypeAnimation,
	) {
		return input, fmt.Errorf("invalid blog_detail.sections[%d].section_type", index)
	}
	if input.SectionName == "" {
		input.SectionName = defaultBlogSectionName(input.SectionType)
	}
	settings, err := normalizeJSONObject(input.Settings, fmt.Sprintf("blog_detail.sections[%d].settings", index))
	if err != nil {
		return input, err
	}
	input.Settings = settings

	switch input.SectionType {
	case BlogSectionTypeHeading:
		if input.Heading == nil {
			input.Heading = &BlogHeadingSectionInput{}
		}
		input.Heading.HeadingText = strings.TrimSpace(input.Heading.HeadingText)
		if input.Heading.HeadingText == "" {
			return input, fmt.Errorf("blog_detail.sections[%d].heading.heading_text is required", index)
		}
		if input.Heading.UnderlineEnabled == nil {
			input.Heading.UnderlineEnabled = boolPtr(false)
		}
	case BlogSectionTypeImage:
		if input.Image == nil {
			input.Image = &BlogImageSectionInput{}
		}
		input.Image.Caption = strings.TrimSpace(input.Image.Caption)
		input.Image.Asset = normalizeBlogUploadPointer(input.Image.Asset)
		if input.Image.Asset == nil {
			return input, fmt.Errorf("blog_detail.sections[%d].image.asset is required", index)
		}
	case BlogSectionTypeTypography:
		if input.Typography == nil {
			input.Typography = &BlogTypographySectionInput{}
		}
		input.Typography.HTMLContent = strings.TrimSpace(input.Typography.HTMLContent)
		input.Typography.TextContent = buildPlainTextFromHTML(input.Typography.HTMLContent, input.Typography.TextContent)
		if input.Typography.TextContent == "" {
			return input, fmt.Errorf("blog_detail.sections[%d].typography.html_content is required", index)
		}
	case BlogSectionTypeAction:
		if input.Action == nil {
			input.Action = &BlogActionSectionInput{}
		}
		input.Action.Text = strings.TrimSpace(input.Action.Text)
		input.Action.ActionType = strings.ToLower(strings.TrimSpace(input.Action.ActionType))
		input.Action.TargetURL = strings.TrimSpace(input.Action.TargetURL)
		if input.Action.Text == "" {
			return input, fmt.Errorf("blog_detail.sections[%d].action.text is required", index)
		}
		if !isAllowed(input.Action.ActionType, BlogActionTypeLink, BlogActionTypeVideo) {
			return input, fmt.Errorf("invalid blog_detail.sections[%d].action.action_type", index)
		}
		if input.Action.TargetURL == "" {
			return input, fmt.Errorf("blog_detail.sections[%d].action.target_url is required", index)
		}
	case BlogSectionTypeVideo:
		if input.Video == nil {
			input.Video = &BlogVideoSectionInput{}
		}
		input.Video.YouTubeURL = strings.TrimSpace(input.Video.YouTubeURL)
		input.Video.Caption = strings.TrimSpace(input.Video.Caption)
		if input.Video.YouTubeURL == "" {
			return input, fmt.Errorf("blog_detail.sections[%d].video.youtube_url is required", index)
		}
	case BlogSectionTypeAnimation:
		if input.Animation == nil {
			input.Animation = &BlogAnimationSectionInput{}
		}
		input.Animation.Navigation = strings.ToLower(strings.TrimSpace(input.Animation.Navigation))
		input.Animation.ImagePosition = strings.ToLower(strings.TrimSpace(input.Animation.ImagePosition))
		if !isAllowed(input.Animation.Navigation, BlogAnimationNavigationVertical, BlogAnimationNavigationHorizontal) {
			return input, fmt.Errorf("invalid blog_detail.sections[%d].animation.navigation", index)
		}
		if !isAllowed(input.Animation.ImagePosition, BlogAnimationImagePositionLeft, BlogAnimationImagePositionRight) {
			return input, fmt.Errorf("invalid blog_detail.sections[%d].animation.image_position", index)
		}
		if len(input.Animation.Items) == 0 {
			return input, fmt.Errorf("blog_detail.sections[%d].animation.items is required", index)
		}
		for itemIndex := range input.Animation.Items {
			item := &input.Animation.Items[itemIndex]
			if item.ID != nil && *item.ID <= 0 {
				return input, fmt.Errorf("blog_detail.sections[%d].animation.items[%d].id must be a positive integer", index, itemIndex)
			}
			item.SortOrder = itemIndex
			item.Heading = strings.TrimSpace(item.Heading)
			item.SubHeading = strings.TrimSpace(item.SubHeading)
			item.Description = strings.TrimSpace(item.Description)
			item.Image = normalizeBlogUploadPointer(item.Image)
			if item.Heading == "" {
				return input, fmt.Errorf("blog_detail.sections[%d].animation.items[%d].heading is required", index, itemIndex)
			}
			if item.SubHeading == "" {
				return input, fmt.Errorf("blog_detail.sections[%d].animation.items[%d].sub_heading is required", index, itemIndex)
			}
			if item.Description == "" {
				return input, fmt.Errorf("blog_detail.sections[%d].animation.items[%d].description is required", index, itemIndex)
			}
			if item.Image == nil {
				return input, fmt.Errorf("blog_detail.sections[%d].animation.items[%d].image is required", index, itemIndex)
			}
		}
	}
	return input, nil
}

func (s *BlogService) getBlogContentDetail(blogID int) (*BlogContentDetailResponse, error) {
	var detail BlogContentDetail
	if err := s.DB.Where("blog_id = ?", blogID).First(&detail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var sections []BlogSection
	if err := s.DB.Where("blog_detail_id = ?", detail.ID).Order("sort_order ASC").Order("id ASC").Find(&sections).Error; err != nil {
		return nil, err
	}
	responses := make([]BlogSectionResponse, 0, len(sections))
	if len(sections) == 0 {
		return buildBlogContentDetailResponse(detail, responses), nil
	}
	sectionIDs := make([]int, 0, len(sections))
	for _, section := range sections {
		sectionIDs = append(sectionIDs, section.ID)
	}

	headings, err := loadBlogHeadingModules(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	images, err := loadBlogImageModules(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	typography, err := loadBlogTypographyModules(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	actions, err := loadBlogActionModules(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	videos, err := loadBlogVideoModules(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	animations, err := loadBlogAnimationModules(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	animationItems, err := loadBlogAnimationItems(s.DB, blogID, sectionIDs)
	if err != nil {
		return nil, err
	}

	for _, section := range sections {
		response := BlogSectionResponse{
			ID: section.ID, SectionName: section.SectionName, SectionType: section.SectionType,
			SortOrder: section.SortOrder, IsEnabled: section.IsEnabled,
			Settings: normalizeJSONRawMessage(section.Settings), CreatedAt: section.CreatedAt, UpdatedAt: section.UpdatedAt,
		}
		switch section.SectionType {
		case BlogSectionTypeHeading:
			if module, ok := headings[section.ID]; ok {
				response.Heading = &BlogHeadingSectionResponse{HeadingText: module.HeadingText, UnderlineEnabled: module.UnderlineEnabled}
			}
		case BlogSectionTypeImage:
			if module, ok := images[section.ID]; ok {
				response.Image = &BlogImageSectionResponse{Caption: module.Caption}
				if module.ImageURL != "" || module.ImageObjectKey != "" {
					response.Image.Asset = buildBlogSectionAssetResponse(
						blogStoredAsset{FileURL: module.ImageURL, GCPObjectKey: module.ImageObjectKey},
						buildBlogSectionImageFetchURL(blogID, section.ID),
					)
				}
			}
		case BlogSectionTypeTypography:
			if module, ok := typography[section.ID]; ok {
				response.Typography = &BlogTypographySectionResponse{HTMLContent: module.BodyHTML, TextContent: module.BodyText}
			}
		case BlogSectionTypeAction:
			if module, ok := actions[section.ID]; ok {
				response.Action = &BlogActionSectionResponse{Text: module.ActionText, ActionType: module.ActionType, TargetURL: module.TargetURL}
			}
		case BlogSectionTypeVideo:
			if module, ok := videos[section.ID]; ok {
				response.Video = &BlogVideoSectionResponse{YouTubeURL: module.YouTubeURL, Caption: module.Caption}
			}
		case BlogSectionTypeAnimation:
			if module, ok := animations[section.ID]; ok {
				items := animationItems[section.ID]
				if items == nil {
					items = make([]BlogAnimationItemResponse, 0)
				}
				response.Animation = &BlogAnimationSectionResponse{Navigation: module.Navigation, ImagePosition: module.ImagePosition, Items: items}
			}
		}
		responses = append(responses, response)
	}
	return buildBlogContentDetailResponse(detail, responses), nil
}

func buildBlogContentDetailResponse(detail BlogContentDetail, sections []BlogSectionResponse) *BlogContentDetailResponse {
	return &BlogContentDetailResponse{
		ID: detail.ID, BlogID: detail.BlogID, Settings: normalizeJSONRawMessage(detail.Settings),
		SchemaVersion: detail.SchemaVersion, Sections: sections, CreatedBy: detail.CreatedBy,
		UpdatedBy: detail.UpdatedBy, CreatedAt: detail.CreatedAt, UpdatedAt: detail.UpdatedAt,
	}
}

func loadBlogHeadingModules(db *gorm.DB, ids []int) (map[int]BlogSectionHeadingModule, error) {
	var rows []BlogSectionHeadingModule
	err := db.Where("blog_section_id IN ?", ids).Find(&rows).Error
	return mapBlogRows(rows, func(row BlogSectionHeadingModule) int { return row.BlogSectionID }), err
}
func loadBlogImageModules(db *gorm.DB, ids []int) (map[int]BlogSectionImageModule, error) {
	var rows []BlogSectionImageModule
	err := db.Where("blog_section_id IN ?", ids).Find(&rows).Error
	return mapBlogRows(rows, func(row BlogSectionImageModule) int { return row.BlogSectionID }), err
}
func loadBlogTypographyModules(db *gorm.DB, ids []int) (map[int]BlogSectionTypographyModule, error) {
	var rows []BlogSectionTypographyModule
	err := db.Where("blog_section_id IN ?", ids).Find(&rows).Error
	return mapBlogRows(rows, func(row BlogSectionTypographyModule) int { return row.BlogSectionID }), err
}
func loadBlogActionModules(db *gorm.DB, ids []int) (map[int]BlogSectionActionModule, error) {
	var rows []BlogSectionActionModule
	err := db.Where("blog_section_id IN ?", ids).Find(&rows).Error
	return mapBlogRows(rows, func(row BlogSectionActionModule) int { return row.BlogSectionID }), err
}
func loadBlogVideoModules(db *gorm.DB, ids []int) (map[int]BlogSectionVideoModule, error) {
	var rows []BlogSectionVideoModule
	err := db.Where("blog_section_id IN ?", ids).Find(&rows).Error
	return mapBlogRows(rows, func(row BlogSectionVideoModule) int { return row.BlogSectionID }), err
}
func loadBlogAnimationModules(db *gorm.DB, ids []int) (map[int]BlogSectionAnimationModule, error) {
	var rows []BlogSectionAnimationModule
	err := db.Where("blog_section_id IN ?", ids).Find(&rows).Error
	return mapBlogRows(rows, func(row BlogSectionAnimationModule) int { return row.BlogSectionID }), err
}

func mapBlogRows[T any](rows []T, key func(T) int) map[int]T {
	result := make(map[int]T, len(rows))
	for _, row := range rows {
		result[key(row)] = row
	}
	return result
}

func loadBlogAnimationItems(db *gorm.DB, blogID int, sectionIDs []int) (map[int][]BlogAnimationItemResponse, error) {
	var rows []BlogAnimationItem
	if err := db.Where("blog_section_id IN ?", sectionIDs).Order("blog_section_id ASC").Order("sort_order ASC").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int][]BlogAnimationItemResponse)
	for _, row := range rows {
		item := BlogAnimationItemResponse{ID: row.ID, SortOrder: row.SortOrder, Heading: row.Heading, SubHeading: row.SubHeading, Description: row.Description}
		if row.ImageURL != "" || row.ImageObjectKey != "" {
			item.Image = buildBlogSectionAssetResponse(
				blogStoredAsset{FileURL: row.ImageURL, GCPObjectKey: row.ImageObjectKey},
				buildBlogAnimationItemImageFetchURL(blogID, row.BlogSectionID, row.ID),
			)
		}
		result[row.BlogSectionID] = append(result[row.BlogSectionID], item)
	}
	return result, nil
}

func (s *BlogService) saveBlogContentDetail(tx *gorm.DB, blogID int, input *SaveBlogDetailRequest, userID *int) ([]string, []blogStoredObject, error) {
	if input == nil {
		return nil, nil, nil
	}
	normalized, err := normalizeSaveBlogDetailRequest(input)
	if err != nil {
		return nil, nil, err
	}
	var detail BlogContentDetail
	if err := tx.Where("blog_id = ?", blogID).First(&detail).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		detail = BlogContentDetail{BlogID: blogID, Settings: normalized.Settings, SchemaVersion: 1, CreatedBy: userID, UpdatedBy: userID}
		if err := tx.Create(&detail).Error; err != nil {
			return nil, nil, err
		}
	} else {
		detail.Settings = normalized.Settings
		detail.UpdatedBy = userID
		if err := tx.Save(&detail).Error; err != nil {
			return nil, nil, err
		}
	}

	candidates, err := collectBlogDetailStoredObjects(tx, detail.ID)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Where("blog_detail_id = ?", detail.ID).Delete(&BlogSection{}).Error; err != nil {
		return nil, nil, err
	}
	uploaded := make([]string, 0)
	reused := make(map[string]struct{})

	for sectionIndex, section := range normalized.Sections {
		row := BlogSection{
			BlogDetailID: detail.ID, SectionName: section.SectionName, SectionType: section.SectionType,
			SortOrder: sectionIndex, IsEnabled: section.IsEnabled, Settings: section.Settings,
		}
		if err := tx.Create(&row).Error; err != nil {
			return uploaded, nil, err
		}
		switch section.SectionType {
		case BlogSectionTypeHeading:
			module := BlogSectionHeadingModule{BlogSectionID: row.ID, HeadingText: section.Heading.HeadingText, UnderlineEnabled: boolValue(section.Heading.UnderlineEnabled, false)}
			if err := tx.Create(&module).Error; err != nil {
				return uploaded, nil, err
			}
		case BlogSectionTypeImage:
			asset, uploadedObject, err := s.storeBlogUploadInput(
				s.sectionImageObjectName(blogID, row.ID, section.Image.Asset.FileName, section.Image.Asset.MimeType),
				*section.Image.Asset, "section image",
			)
			if err != nil {
				return uploaded, nil, err
			}
			if uploadedObject != "" {
				uploaded = append(uploaded, uploadedObject)
			}
			reused[blogStoredObjectFingerprint(blogStoredObject{ObjectKey: asset.GCPObjectKey, StorageURL: asset.FileURL})] = struct{}{}
			module := BlogSectionImageModule{BlogSectionID: row.ID, ImageURL: asset.FileURL, ImageObjectKey: asset.GCPObjectKey, Caption: section.Image.Caption}
			if err := tx.Create(&module).Error; err != nil {
				return uploaded, nil, err
			}
		case BlogSectionTypeTypography:
			module := BlogSectionTypographyModule{BlogSectionID: row.ID, BodyHTML: section.Typography.HTMLContent, BodyText: section.Typography.TextContent}
			if err := tx.Create(&module).Error; err != nil {
				return uploaded, nil, err
			}
		case BlogSectionTypeAction:
			module := BlogSectionActionModule{BlogSectionID: row.ID, ActionText: section.Action.Text, ActionType: section.Action.ActionType, TargetURL: section.Action.TargetURL}
			if err := tx.Create(&module).Error; err != nil {
				return uploaded, nil, err
			}
		case BlogSectionTypeVideo:
			module := BlogSectionVideoModule{BlogSectionID: row.ID, YouTubeURL: section.Video.YouTubeURL, Caption: section.Video.Caption}
			if err := tx.Create(&module).Error; err != nil {
				return uploaded, nil, err
			}
		case BlogSectionTypeAnimation:
			module := BlogSectionAnimationModule{BlogSectionID: row.ID, Navigation: section.Animation.Navigation, ImagePosition: section.Animation.ImagePosition}
			if err := tx.Create(&module).Error; err != nil {
				return uploaded, nil, err
			}
			for itemIndex, item := range section.Animation.Items {
				itemRow := BlogAnimationItem{BlogSectionID: row.ID, SortOrder: itemIndex, Heading: item.Heading, SubHeading: item.SubHeading, Description: item.Description}
				if err := tx.Create(&itemRow).Error; err != nil {
					return uploaded, nil, err
				}
				asset, uploadedObject, err := s.storeBlogUploadInput(
					s.animationItemImageObjectName(blogID, row.ID, itemRow.ID, item.Image.FileName, item.Image.MimeType),
					*item.Image, "animation item image",
				)
				if err != nil {
					return uploaded, nil, err
				}
				if uploadedObject != "" {
					uploaded = append(uploaded, uploadedObject)
				}
				reused[blogStoredObjectFingerprint(blogStoredObject{ObjectKey: asset.GCPObjectKey, StorageURL: asset.FileURL})] = struct{}{}
				if err := tx.Model(&itemRow).Updates(map[string]any{"image_url": asset.FileURL, "image_object_key": asset.GCPObjectKey}).Error; err != nil {
					return uploaded, nil, err
				}
			}
		}
	}
	cleanup := make([]blogStoredObject, 0)
	for _, candidate := range candidates {
		if _, ok := reused[blogStoredObjectFingerprint(candidate)]; !ok {
			cleanup = append(cleanup, candidate)
		}
	}
	return uploaded, cleanup, nil
}

func collectBlogStoredObjects(tx *gorm.DB, blog Blog) ([]blogStoredObject, error) {
	objects := make([]blogStoredObject, 0)
	if blog.CoverImageURL != "" || blog.CoverImageObjectKey != "" {
		objects = append(objects, blogStoredObject{ObjectKey: blog.CoverImageObjectKey, StorageURL: blog.CoverImageURL})
	}
	var detail BlogContentDetail
	if err := tx.Where("blog_id = ?", blog.ID).First(&detail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return objects, nil
		}
		return nil, err
	}
	sectionObjects, err := collectBlogDetailStoredObjects(tx, detail.ID)
	return append(objects, sectionObjects...), err
}

func collectBlogDetailStoredObjects(tx *gorm.DB, detailID int) ([]blogStoredObject, error) {
	type mediaRow struct {
		ImageURL       string
		ImageObjectKey string
	}
	var imageRows []mediaRow
	if err := tx.Table("blog_section_image_modules").
		Select("blog_section_image_modules.image_url, blog_section_image_modules.image_object_key").
		Joins("JOIN blog_sections ON blog_sections.id = blog_section_image_modules.blog_section_id").
		Where("blog_sections.blog_detail_id = ?", detailID).Scan(&imageRows).Error; err != nil {
		return nil, err
	}
	var itemRows []mediaRow
	if err := tx.Table("blog_animation_items").
		Select("blog_animation_items.image_url, blog_animation_items.image_object_key").
		Joins("JOIN blog_sections ON blog_sections.id = blog_animation_items.blog_section_id").
		Where("blog_sections.blog_detail_id = ?", detailID).Scan(&itemRows).Error; err != nil {
		return nil, err
	}
	objects := make([]blogStoredObject, 0, len(imageRows)+len(itemRows))
	for _, row := range append(imageRows, itemRows...) {
		if row.ImageURL != "" || row.ImageObjectKey != "" {
			objects = append(objects, blogStoredObject{ObjectKey: row.ImageObjectKey, StorageURL: row.ImageURL})
		}
	}
	return objects, nil
}

func buildBlogSectionAssetResponse(asset blogStoredAsset, fetchURL string) *BlogSectionAssetResponse {
	return &BlogSectionAssetResponse{FileURL: fetchURL, FetchURL: fetchURL, StorageURI: asset.FileURL, GCPObjectKey: asset.GCPObjectKey}
}

func normalizeJSONObject(value JSONRawMessage, fieldName string) (JSONRawMessage, error) {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" || trimmed == "null" {
		return JSONRawMessage(`{}`), nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("%s must be valid JSON", fieldName)
	}
	if _, ok := decoded.(map[string]any); !ok {
		return nil, fmt.Errorf("%s must be a JSON object", fieldName)
	}
	return JSONRawMessage(trimmed), nil
}

func normalizeJSONRawMessage(value JSONRawMessage) JSONRawMessage {
	if len(value) == 0 {
		return JSONRawMessage(`{}`)
	}
	return value
}

func defaultBlogSectionName(sectionType string) string {
	names := map[string]string{
		BlogSectionTypeHeading: "Heading Module", BlogSectionTypeImage: "Image Module",
		BlogSectionTypeTypography: "Typography Module", BlogSectionTypeAction: "Action Module",
		BlogSectionTypeVideo: "Video Module", BlogSectionTypeAnimation: "Animation Module",
	}
	if name := names[sectionType]; name != "" {
		return name
	}
	return "Content Section"
}

func buildPlainTextFromHTML(html, fallback string) string {
	plain := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, " ")
	plain = strings.Join(strings.Fields(plain), " ")
	if plain != "" {
		return plain
	}
	return strings.Join(strings.Fields(fallback), " ")
}

func normalizeBlogUploadPointer(input *BlogUploadInput) *BlogUploadInput {
	if input == nil {
		return nil
	}
	cleaned := sanitizeUploadInput(*input)
	if isEmptyBlogUploadInput(cleaned) {
		return nil
	}
	return &cleaned
}

func sanitizeUploadInput(value BlogUploadInput) BlogUploadInput {
	value.FileName = strings.TrimSpace(value.FileName)
	value.MimeType = strings.TrimSpace(value.MimeType)
	value.DataBase64 = strings.TrimSpace(value.DataBase64)
	value.FileURL = strings.TrimSpace(value.FileURL)
	value.StorageURI = strings.TrimSpace(value.StorageURI)
	value.ObjectKey = strings.TrimSpace(value.ObjectKey)
	value.GCPObjectKey = strings.TrimSpace(value.GCPObjectKey)
	return value
}

func isEmptyBlogUploadInput(value BlogUploadInput) bool {
	return value.FileName == "" && value.MimeType == "" && value.DataBase64 == "" && len(value.Content) == 0 &&
		value.FileURL == "" && value.StorageURI == "" && value.ObjectKey == "" && value.GCPObjectKey == ""
}

func isAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

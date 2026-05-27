package pages

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"nordikcsaaapi/internal/util"

	"gorm.io/gorm"
)

var (
	ErrPageDocumentNotFound = errors.New("page document not found")
)

type pageDocumentReferenceRow struct {
	ID           int
	FileURL      string
	GCPObjectKey string
}

type pageStoredObject struct {
	ObjectKey  string
	StorageURL string
}

func normalizeSavePageDetailRequest(input *SavePageDetailRequest) (*SavePageDetailRequest, error) {
	if input == nil {
		return nil, nil
	}

	next := *input
	next.TemplateKey = strings.TrimSpace(next.TemplateKey)
	if next.TemplateKey == "" {
		next.TemplateKey = "default"
	}

	settings, err := normalizeJSONObject(next.Settings, "page_detail.settings")
	if err != nil {
		return nil, err
	}
	next.Settings = settings

	sections := make([]SavePageSectionRequest, 0, len(next.Sections))
	for idx, section := range next.Sections {
		normalizedSection, err := normalizeSavePageSectionRequest(section, idx)
		if err != nil {
			return nil, err
		}
		sections = append(sections, normalizedSection)
	}
	next.Sections = sections

	return &next, nil
}

func normalizeSavePageSectionRequest(input SavePageSectionRequest, index int) (SavePageSectionRequest, error) {
	input.SectionName = strings.TrimSpace(input.SectionName)
	input.SectionType = strings.ToLower(strings.TrimSpace(input.SectionType))
	input.SortOrder = index

	if input.ID != nil && *input.ID <= 0 {
		return input, fmt.Errorf("page_detail.sections[%d].id must be a positive integer", index)
	}
	if !isAllowed(input.SectionType,
		PageSectionTypeHeader,
		PageSectionTypeTypography,
		PageSectionTypeGallery,
		PageSectionTypeDocument,
		PageSectionTypeQuote,
		PageSectionTypeCTABanner,
	) {
		return input, fmt.Errorf("invalid page_detail.sections[%d].section_type", index)
	}
	if input.SectionName == "" {
		input.SectionName = defaultPageSectionName(input.SectionType)
	}

	settings, err := normalizeJSONObject(input.Settings, fmt.Sprintf("page_detail.sections[%d].settings", index))
	if err != nil {
		return input, err
	}
	input.Settings = settings

	switch input.SectionType {
	case PageSectionTypeHeader:
		if input.Header == nil {
			input.Header = &PageHeaderSectionInput{}
		}
		input.Header.MainHeaderText = strings.TrimSpace(input.Header.MainHeaderText)
		input.Header.SubHeaderText = strings.TrimSpace(input.Header.SubHeaderText)
		input.Header.Description = strings.TrimSpace(input.Header.Description)
		input.Header.Hierarchy = strings.ToLower(strings.TrimSpace(input.Header.Hierarchy))
		input.Header.TextAlign = strings.ToLower(strings.TrimSpace(input.Header.TextAlign))
		if input.Header.Hierarchy == "" {
			input.Header.Hierarchy = PageHeaderHierarchyHero
		}
		if !isAllowed(input.Header.Hierarchy, PageHeaderHierarchyHero, PageHeaderHierarchySection) {
			return input, fmt.Errorf("invalid page_detail.sections[%d].header.hierarchy", index)
		}
		if input.Header.TextAlign == "" {
			input.Header.TextAlign = PageTextAlignLeft
		}
		if !isAllowed(input.Header.TextAlign, PageTextAlignLeft, PageTextAlignCenter, PageTextAlignRight) {
			return input, fmt.Errorf("invalid page_detail.sections[%d].header.text_align", index)
		}
		if input.Header.UnderlineEnabled == nil {
			input.Header.UnderlineEnabled = boolPtr(false)
		}
		if boolValue(input.Header.UnderlineEnabled, false) && input.Header.Hierarchy != PageHeaderHierarchySection {
			return input, fmt.Errorf("page_detail.sections[%d].header.underline_enabled can only be true when hierarchy is h2_section", index)
		}
	case PageSectionTypeTypography:
		if input.Typography == nil {
			input.Typography = &PageTypographySectionInput{}
		}
		input.Typography.HTMLContent = strings.TrimSpace(input.Typography.HTMLContent)
		input.Typography.TextContent = strings.TrimSpace(input.Typography.TextContent)
		input.Typography.TextAlign = strings.ToLower(strings.TrimSpace(input.Typography.TextAlign))
		if input.Typography.TextAlign == "" {
			input.Typography.TextAlign = PageTypographyAlignLeft
		}
		if !isAllowed(input.Typography.TextAlign, PageTypographyAlignLeft, PageTypographyAlignCenter, PageTypographyAlignRight) {
			return input, fmt.Errorf("invalid page_detail.sections[%d].typography.text_align", index)
		}
	case PageSectionTypeGallery:
		if input.Gallery == nil {
			input.Gallery = &PageGallerySectionInput{}
		}
		if input.Gallery.GalleryID != nil && *input.Gallery.GalleryID <= 0 {
			return input, fmt.Errorf("page_detail.sections[%d].gallery.gallery_id must be a positive integer", index)
		}
		input.Gallery.ViewMode = strings.ToLower(strings.TrimSpace(input.Gallery.ViewMode))
		if input.Gallery.ViewMode == "" {
			input.Gallery.ViewMode = PageGalleryViewGrid
		}
		if !isAllowed(
			input.Gallery.ViewMode,
			PageGalleryViewGrid,
			PageGalleryViewCarousel,
			PageGalleryViewMasonry,
			PageGalleryViewFocus,
			PageGalleryViewIcons,
		) {
			return input, fmt.Errorf("invalid page_detail.sections[%d].gallery.view_mode", index)
		}
		if input.Gallery.ShowTitleDescription == nil {
			input.Gallery.ShowTitleDescription = boolPtr(true)
		}
		if input.Gallery.AutoScrollEnabled == nil {
			input.Gallery.AutoScrollEnabled = boolPtr(false)
		}
	case PageSectionTypeDocument:
		if input.Documents == nil {
			input.Documents = &PageDocumentsSectionInput{}
		}
		items := make([]PageDocumentInput, 0, len(input.Documents.Items))
		for docIdx, item := range input.Documents.Items {
			normalizedItem, err := normalizePageDocumentInput(item, index, docIdx)
			if err != nil {
				return input, err
			}
			items = append(items, normalizedItem)
		}
		input.Documents.Items = items
	case PageSectionTypeQuote:
		if input.Quote == nil {
			input.Quote = &PageQuoteSectionInput{}
		}
		input.Quote.QuoteContent = strings.TrimSpace(input.Quote.QuoteContent)
		input.Quote.Attribution = strings.TrimSpace(input.Quote.Attribution)
	case PageSectionTypeCTABanner:
		if input.CTABanner == nil {
			input.CTABanner = &PageCTABannerSectionInput{}
		}
		input.CTABanner.BannerHeading = strings.TrimSpace(input.CTABanner.BannerHeading)
		input.CTABanner.BannerMessage = strings.TrimSpace(input.CTABanner.BannerMessage)
		input.CTABanner.ButtonText = strings.TrimSpace(input.CTABanner.ButtonText)
		input.CTABanner.ButtonURL = strings.TrimSpace(input.CTABanner.ButtonURL)
	}

	return input, nil
}

func normalizePageDocumentInput(input PageDocumentInput, sectionIndex int, documentIndex int) (PageDocumentInput, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Description = strings.TrimSpace(input.Description)
	input.OriginalFileName = strings.TrimSpace(input.OriginalFileName)
	input.FileName = strings.TrimSpace(input.FileName)
	input.MimeType = strings.TrimSpace(input.MimeType)
	input.DataBase64 = strings.TrimSpace(input.DataBase64)
	input.FileURL = strings.TrimSpace(input.FileURL)
	input.StorageURI = strings.TrimSpace(input.StorageURI)
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.GCPObjectKey = strings.TrimSpace(input.GCPObjectKey)

	if input.ID != nil && *input.ID <= 0 {
		return input, fmt.Errorf("page_detail.sections[%d].documents.items[%d].id must be a positive integer", sectionIndex, documentIndex)
	}

	if input.DisplayName == "" {
		input.DisplayName = suggestPageDocumentDisplayName(input)
	}

	if err := validatePageDocumentInput(input, sectionIndex, documentIndex); err != nil {
		return input, err
	}

	return input, nil
}

func validatePageDocumentInput(input PageDocumentInput, sectionIndex int, documentIndex int) error {
	referenceURL := input.StorageURI
	if referenceURL == "" {
		referenceURL = input.FileURL
	}

	if input.ID == nil && len(input.Content) == 0 && input.DataBase64 == "" && referenceURL == "" && input.ObjectKey == "" && input.GCPObjectKey == "" {
		return fmt.Errorf("page_detail.sections[%d].documents.items[%d] is missing both uploaded file and file_url", sectionIndex, documentIndex)
	}

	mimeType := strings.ToLower(strings.TrimSpace(input.MimeType))
	if mimeType != "" && !isAllowed(mimeType,
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	) {
		return fmt.Errorf("page_detail.sections[%d].documents.items[%d] must be a supported document file", sectionIndex, documentIndex)
	}

	if mimeType == "" {
		ext := strings.ToLower(util.ExtFromFilenameOrMime(input.FileName, input.MimeType))
		if ext == "" {
			ext = strings.ToLower(path.Ext(input.OriginalFileName))
		}
		if ext == "" {
			ext = strings.ToLower(path.Ext(referenceURL))
		}
		if ext != "" && !isAllowed(ext, ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx") {
			return fmt.Errorf("page_detail.sections[%d].documents.items[%d] must be a supported document file", sectionIndex, documentIndex)
		}
	}

	return nil
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

func defaultPageSectionName(sectionType string) string {
	switch sectionType {
	case PageSectionTypeHeader:
		return "Header Module"
	case PageSectionTypeTypography:
		return "Typography"
	case PageSectionTypeGallery:
		return "Gallery Module"
	case PageSectionTypeDocument:
		return "Document Module"
	case PageSectionTypeQuote:
		return "Quote Module"
	case PageSectionTypeCTABanner:
		return "CTA Banner"
	default:
		return "Content Section"
	}
}

func suggestPageDocumentDisplayName(input PageDocumentInput) string {
	candidates := []string{
		input.OriginalFileName,
		input.FileName,
		path.Base(strings.TrimSpace(input.ObjectKey)),
		path.Base(strings.TrimSpace(input.GCPObjectKey)),
		path.Base(strings.TrimSpace(input.StorageURI)),
		path.Base(strings.TrimSpace(input.FileURL)),
	}

	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == "." || candidate == "/" {
			continue
		}
		candidate = strings.TrimSuffix(candidate, path.Ext(candidate))
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}

	return "Document"
}

func (s *PageService) getPageContentDetail(pageID int) (*PageContentDetailResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var detail PageContentDetail
	if err := s.DB.Where("page_id = ?", pageID).First(&detail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultPageContentDetailResponse(pageID), nil
		}
		return nil, err
	}

	var sections []PageSection
	if err := s.DB.
		Where("page_detail_id = ?", detail.ID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&sections).Error; err != nil {
		return nil, err
	}

	response := &PageContentDetailResponse{
		ID:            detail.ID,
		PageID:        detail.PageID,
		TemplateKey:   detail.TemplateKey,
		Settings:      normalizeJSONRawMessage(detail.Settings),
		SchemaVersion: detail.SchemaVersion,
		Sections:      make([]PageSectionResponse, 0, len(sections)),
		CreatedBy:     detail.CreatedBy,
		UpdatedBy:     detail.UpdatedBy,
		CreatedAt:     detail.CreatedAt,
		UpdatedAt:     detail.UpdatedAt,
	}

	if len(sections) == 0 {
		return response, nil
	}

	sectionIDs := make([]int, 0, len(sections))
	for _, section := range sections {
		sectionIDs = append(sectionIDs, section.ID)
	}

	headersBySection, err := loadPageSectionHeaders(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	typographyBySection, err := loadPageSectionTypography(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	galleriesBySection, err := loadPageSectionGalleries(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	quotesBySection, err := loadPageSectionQuotes(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	ctaBySection, err := loadPageSectionCTABanners(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}
	documentsBySection, err := loadPageSectionDocuments(s.DB, sectionIDs)
	if err != nil {
		return nil, err
	}

	for _, section := range sections {
		item := PageSectionResponse{
			ID:          section.ID,
			SectionName: section.SectionName,
			SectionType: section.SectionType,
			SortOrder:   section.SortOrder,
			IsEnabled:   section.IsEnabled,
			Settings:    normalizeJSONRawMessage(section.Settings),
			CreatedAt:   section.CreatedAt,
			UpdatedAt:   section.UpdatedAt,
		}

		if header, ok := headersBySection[section.ID]; ok {
			item.Header = &PageHeaderSectionResponse{
				MainHeaderText:   header.MainHeaderText,
				SubHeaderText:    header.SubHeaderText,
				Description:      header.Description,
				Hierarchy:        header.Hierarchy,
				TextAlign:        header.TextAlign,
				UnderlineEnabled: header.UnderlineEnabled,
			}
		}
		if typography, ok := typographyBySection[section.ID]; ok {
			item.Typography = &PageTypographySectionResponse{
				HTMLContent: typography.BodyHTML,
				TextContent: typography.BodyText,
				TextAlign:   typography.TextAlign,
			}
		}
		if gallery, ok := galleriesBySection[section.ID]; ok {
			item.Gallery = &PageGallerySectionResponse{
				GalleryID:            gallery.GalleryID,
				ViewMode:             gallery.ViewMode,
				ShowTitleDescription: gallery.ShowTitleDescription,
				AutoScrollEnabled:    gallery.AutoScrollEnabled,
			}
		}
		if quote, ok := quotesBySection[section.ID]; ok {
			item.Quote = &PageQuoteSectionResponse{
				QuoteContent: quote.QuoteContent,
				Attribution:  quote.Attribution,
			}
		}
		if cta, ok := ctaBySection[section.ID]; ok {
			item.CTABanner = &PageCTABannerSectionResponse{
				BannerHeading: cta.BannerHeading,
				BannerMessage: cta.BannerMessage,
				ButtonText:    cta.ButtonText,
				ButtonURL:     cta.ButtonURL,
				OpenInNewTab:  cta.OpenInNewTab,
			}
		}
		if documents, ok := documentsBySection[section.ID]; ok {
			item.Documents = &PageDocumentsSectionResponse{Items: documents}
		}

		response.Sections = append(response.Sections, item)
	}

	return response, nil
}

func defaultPageContentDetailResponse(pageID int) *PageContentDetailResponse {
	return &PageContentDetailResponse{
		ID:            0,
		PageID:        pageID,
		TemplateKey:   "default",
		Settings:      JSONRawMessage(`{}`),
		SchemaVersion: 1,
		Sections:      make([]PageSectionResponse, 0),
	}
}

func normalizeJSONRawMessage(value JSONRawMessage) JSONRawMessage {
	if len(value) == 0 {
		return JSONRawMessage(`{}`)
	}
	return value
}

func loadPageSectionHeaders(db *gorm.DB, sectionIDs []int) (map[int]PageSectionHeaderModule, error) {
	var rows []PageSectionHeaderModule
	if err := db.Where("page_section_id IN ?", sectionIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make(map[int]PageSectionHeaderModule, len(rows))
	for _, row := range rows {
		items[row.PageSectionID] = row
	}
	return items, nil
}

func loadPageSectionTypography(db *gorm.DB, sectionIDs []int) (map[int]PageSectionTypographyModule, error) {
	var rows []PageSectionTypographyModule
	if err := db.Where("page_section_id IN ?", sectionIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make(map[int]PageSectionTypographyModule, len(rows))
	for _, row := range rows {
		items[row.PageSectionID] = row
	}
	return items, nil
}

func loadPageSectionGalleries(db *gorm.DB, sectionIDs []int) (map[int]PageSectionGalleryModule, error) {
	var rows []PageSectionGalleryModule
	if err := db.Where("page_section_id IN ?", sectionIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make(map[int]PageSectionGalleryModule, len(rows))
	for _, row := range rows {
		items[row.PageSectionID] = row
	}
	return items, nil
}

func loadPageSectionQuotes(db *gorm.DB, sectionIDs []int) (map[int]PageSectionQuoteModule, error) {
	var rows []PageSectionQuoteModule
	if err := db.Where("page_section_id IN ?", sectionIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make(map[int]PageSectionQuoteModule, len(rows))
	for _, row := range rows {
		items[row.PageSectionID] = row
	}
	return items, nil
}

func loadPageSectionCTABanners(db *gorm.DB, sectionIDs []int) (map[int]PageSectionCTABannerModule, error) {
	var rows []PageSectionCTABannerModule
	if err := db.Where("page_section_id IN ?", sectionIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make(map[int]PageSectionCTABannerModule, len(rows))
	for _, row := range rows {
		items[row.PageSectionID] = row
	}
	return items, nil
}

func loadPageSectionDocuments(db *gorm.DB, sectionIDs []int) (map[int][]PageDocumentResponse, error) {
	if len(sectionIDs) == 0 {
		return map[int][]PageDocumentResponse{}, nil
	}

	type joinedRow struct {
		PageSectionID    int       `gorm:"column:page_section_id"`
		DocumentID       int       `gorm:"column:document_id"`
		DisplayName      string    `gorm:"column:display_name"`
		Description      string    `gorm:"column:description"`
		OriginalFileName string    `gorm:"column:original_file_name"`
		FileURL          string    `gorm:"column:file_url"`
		GCPObjectKey     string    `gorm:"column:gcp_object_key"`
		MimeType         string    `gorm:"column:mime_type"`
		FileSize         int64     `gorm:"column:file_size"`
		SortOrder        int       `gorm:"column:sort_order"`
		CreatedAt        time.Time `gorm:"column:created_at"`
		UpdatedAt        time.Time `gorm:"column:updated_at"`
	}

	var rows []joinedRow
	if err := db.Table("page_section_documents").
		Select(`
			page_section_documents.page_section_id,
			documents.id AS document_id,
			COALESCE(NULLIF(TRIM(page_section_documents.display_name_override), ''), documents.display_name) AS display_name,
			COALESCE(documents.description, '') AS description,
			COALESCE(documents.original_file_name, '') AS original_file_name,
			documents.file_url,
			COALESCE(documents.gcp_object_key, '') AS gcp_object_key,
			COALESCE(documents.mime_type, '') AS mime_type,
			COALESCE(documents.file_size, 0) AS file_size,
			page_section_documents.sort_order,
			documents.created_at,
			documents.updated_at
		`).
		Joins("JOIN documents ON documents.id = page_section_documents.document_id").
		Where("page_section_documents.page_section_id IN ?", sectionIDs).
		Order("page_section_documents.page_section_id ASC").
		Order("page_section_documents.sort_order ASC").
		Order("page_section_documents.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make(map[int][]PageDocumentResponse)
	for _, row := range rows {
		items[row.PageSectionID] = append(items[row.PageSectionID], PageDocumentResponse{
			ID:               row.DocumentID,
			DisplayName:      row.DisplayName,
			Description:      row.Description,
			OriginalFileName: row.OriginalFileName,
			FileName:         buildPageDocumentFileName(row.DisplayName, row.OriginalFileName, row.GCPObjectKey, row.FileURL, row.MimeType),
			FileURL:          buildPageDocumentFetchURL(row.DocumentID),
			FetchURL:         buildPageDocumentFetchURL(row.DocumentID),
			StorageURI:       row.FileURL,
			GCPObjectKey:     row.GCPObjectKey,
			MimeType:         row.MimeType,
			FileSize:         row.FileSize,
			SortOrder:        row.SortOrder,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}

	return items, nil
}

func (s *PageService) savePageContentDetail(tx *gorm.DB, pageID int, input *SavePageDetailRequest, userID *int) ([]string, []pageStoredObject, error) {
	if input == nil {
		return nil, nil, nil
	}

	normalized, err := normalizeSavePageDetailRequest(input)
	if err != nil {
		return nil, nil, err
	}

	var detail PageContentDetail
	if err := tx.Where("page_id = ?", pageID).First(&detail).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		detail = PageContentDetail{
			PageID:        pageID,
			TemplateKey:   normalized.TemplateKey,
			Settings:      normalized.Settings,
			SchemaVersion: 1,
			CreatedBy:     userID,
			UpdatedBy:     userID,
		}
		if err := tx.Create(&detail).Error; err != nil {
			return nil, nil, err
		}
	} else {
		detail.TemplateKey = normalized.TemplateKey
		detail.Settings = normalized.Settings
		detail.UpdatedBy = userID
		if err := tx.Save(&detail).Error; err != nil {
			return nil, nil, err
		}
	}

	candidateDocuments, err := s.loadPageDetailDocumentReferences(tx, detail.ID)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Where("page_detail_id = ?", detail.ID).Delete(&PageSection{}).Error; err != nil {
		return nil, nil, err
	}

	uploadedObjects := make([]string, 0)
	cleanupObjects := make([]pageStoredObject, 0)
	reusedDocumentIDs := make(map[int]struct{})

	for idx, section := range normalized.Sections {
		row := PageSection{
			PageDetailID: detail.ID,
			SectionName:  section.SectionName,
			SectionType:  section.SectionType,
			SortOrder:    idx,
			IsEnabled:    section.IsEnabled,
			Settings:     section.Settings,
		}
		if err := tx.Create(&row).Error; err != nil {
			return nil, nil, err
		}

		switch section.SectionType {
		case PageSectionTypeHeader:
			module := PageSectionHeaderModule{
				PageSectionID:    row.ID,
				MainHeaderText:   section.Header.MainHeaderText,
				SubHeaderText:    section.Header.SubHeaderText,
				Description:      section.Header.Description,
				Hierarchy:        section.Header.Hierarchy,
				TextAlign:        section.Header.TextAlign,
				UnderlineEnabled: boolValue(section.Header.UnderlineEnabled, false),
			}
			if err := tx.Create(&module).Error; err != nil {
				return nil, nil, err
			}
		case PageSectionTypeTypography:
			module := PageSectionTypographyModule{
				PageSectionID: row.ID,
				BodyHTML:      section.Typography.HTMLContent,
				BodyText:      section.Typography.TextContent,
				TextAlign:     section.Typography.TextAlign,
			}
			if err := tx.Create(&module).Error; err != nil {
				return nil, nil, err
			}
		case PageSectionTypeGallery:
			module := PageSectionGalleryModule{
				PageSectionID:        row.ID,
				GalleryID:            section.Gallery.GalleryID,
				ViewMode:             section.Gallery.ViewMode,
				ShowTitleDescription: boolValue(section.Gallery.ShowTitleDescription, true),
				AutoScrollEnabled:    boolValue(section.Gallery.AutoScrollEnabled, false),
			}
			if err := tx.Create(&module).Error; err != nil {
				return nil, nil, err
			}
		case PageSectionTypeQuote:
			module := PageSectionQuoteModule{
				PageSectionID: row.ID,
				QuoteContent:  section.Quote.QuoteContent,
				Attribution:   section.Quote.Attribution,
			}
			if err := tx.Create(&module).Error; err != nil {
				return nil, nil, err
			}
		case PageSectionTypeCTABanner:
			module := PageSectionCTABannerModule{
				PageSectionID: row.ID,
				BannerHeading: section.CTABanner.BannerHeading,
				BannerMessage: section.CTABanner.BannerMessage,
				ButtonText:    section.CTABanner.ButtonText,
				ButtonURL:     section.CTABanner.ButtonURL,
				OpenInNewTab:  section.CTABanner.OpenInNewTab,
			}
			if err := tx.Create(&module).Error; err != nil {
				return nil, nil, err
			}
		case PageSectionTypeDocument:
			for docIdx, item := range section.Documents.Items {
				documentRow, uploadedObject, cleanupObject, err := s.upsertPageDocument(tx, item, userID)
				if err != nil {
					return nil, nil, err
				}
				if uploadedObject != "" {
					uploadedObjects = append(uploadedObjects, uploadedObject)
				}
				if cleanupObject != nil {
					cleanupObjects = append(cleanupObjects, *cleanupObject)
				}
				reusedDocumentIDs[documentRow.ID] = struct{}{}

				join := PageSectionDocument{
					PageSectionID: row.ID,
					DocumentID:    documentRow.ID,
					SortOrder:     docIdx,
				}
				if err := tx.Create(&join).Error; err != nil {
					return nil, nil, err
				}
			}
		}
	}

	for _, candidate := range candidateDocuments {
		if _, ok := reusedDocumentIDs[candidate.ID]; ok {
			continue
		}

		cleanupObject, err := s.deleteOrphanPageDocument(tx, candidate.ID)
		if err != nil {
			return nil, nil, err
		}
		if cleanupObject != nil {
			cleanupObjects = append(cleanupObjects, *cleanupObject)
		}
	}

	return uploadedObjects, cleanupObjects, nil
}

func (s *PageService) loadPageDetailDocumentReferences(tx *gorm.DB, pageDetailID int) ([]pageDocumentReferenceRow, error) {
	var rows []pageDocumentReferenceRow
	if err := tx.Table("page_sections").
		Select("DISTINCT documents.id, documents.file_url, COALESCE(documents.gcp_object_key, '') AS gcp_object_key").
		Joins("JOIN page_section_documents ON page_section_documents.page_section_id = page_sections.id").
		Joins("JOIN documents ON documents.id = page_section_documents.document_id").
		Where("page_sections.page_detail_id = ?", pageDetailID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *PageService) loadPageDocumentReferencesForPage(tx *gorm.DB, pageID int) ([]pageDocumentReferenceRow, error) {
	var rows []pageDocumentReferenceRow
	if err := tx.Table("page_details").
		Select("DISTINCT documents.id, documents.file_url, COALESCE(documents.gcp_object_key, '') AS gcp_object_key").
		Joins("JOIN page_sections ON page_sections.page_detail_id = page_details.id").
		Joins("JOIN page_section_documents ON page_section_documents.page_section_id = page_sections.id").
		Joins("JOIN documents ON documents.id = page_section_documents.document_id").
		Where("page_details.page_id = ?", pageID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *PageService) upsertPageDocument(tx *gorm.DB, input PageDocumentInput, userID *int) (PageDocument, string, *pageStoredObject, error) {
	var uploadedObject string
	var cleanupObject *pageStoredObject
	referenceOnly := len(input.Content) == 0 && strings.TrimSpace(input.DataBase64) == ""

	if input.ID == nil {
		fileURL, objectKey, fileSize, checksum, uploaded, err := s.storePageDocumentInput(input)
		if err != nil {
			return PageDocument{}, "", nil, err
		}

		row := PageDocument{
			DisplayName:      input.DisplayName,
			Description:      input.Description,
			OriginalFileName: resolvePageDocumentOriginalFileName(input, PageDocument{}),
			GCPObjectKey:     objectKey,
			FileURL:          fileURL,
			MimeType:         input.MimeType,
			FileSize:         fileSize,
			ChecksumSHA256:   checksumPointer(checksum),
			CreatedBy:        userID,
			UpdatedBy:        userID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return PageDocument{}, "", nil, err
		}
		return row, uploaded, nil, nil
	}

	var row PageDocument
	if err := tx.First(&row, *input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PageDocument{}, "", nil, ErrPageDocumentNotFound
		}
		return PageDocument{}, "", nil, err
	}

	oldRow := row

	hasNewFile := len(input.Content) > 0 || strings.TrimSpace(input.DataBase64) != "" ||
		strings.TrimSpace(input.StorageURI) != "" || strings.TrimSpace(input.FileURL) != "" ||
		strings.TrimSpace(input.ObjectKey) != "" || strings.TrimSpace(input.GCPObjectKey) != ""
	if hasNewFile {
		fileURL, objectKey, fileSize, checksum, uploaded, err := s.storePageDocumentInput(input)
		if err != nil {
			return PageDocument{}, "", nil, err
		}
		applyPageDocumentStoredFileFields(&row, oldRow, fileURL, objectKey, fileSize, checksum, referenceOnly)
		uploadedObject = uploaded
	}

	row.DisplayName = input.DisplayName
	row.Description = input.Description
	row.OriginalFileName = resolvePageDocumentOriginalFileName(input, oldRow)
	if strings.TrimSpace(input.MimeType) != "" {
		row.MimeType = input.MimeType
	}
	row.UpdatedBy = userID

	if err := tx.Save(&row).Error; err != nil {
		return PageDocument{}, "", nil, err
	}

	if shouldCleanupStoredObject(oldRow.GCPObjectKey, oldRow.FileURL, row.GCPObjectKey, row.FileURL) {
		cleanupObject = &pageStoredObject{
			ObjectKey:  oldRow.GCPObjectKey,
			StorageURL: oldRow.FileURL,
		}
	}

	return row, uploadedObject, cleanupObject, nil
}

func applyPageDocumentStoredFileFields(row *PageDocument, oldRow PageDocument, fileURL string, objectKey string, fileSize int64, checksum string, referenceOnly bool) {
	if row == nil {
		return
	}

	row.FileURL = fileURL
	row.GCPObjectKey = objectKey

	if !referenceOnly {
		row.FileSize = fileSize
		row.ChecksumSHA256 = checksumPointer(checksum)
		return
	}

	if shouldCleanupStoredObject(oldRow.GCPObjectKey, oldRow.FileURL, objectKey, fileURL) {
		row.FileSize = fileSize
		row.ChecksumSHA256 = checksumPointer(checksum)
		return
	}

	row.FileSize = oldRow.FileSize
	row.ChecksumSHA256 = oldRow.ChecksumSHA256
}

func checksumPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func boolPtr(value bool) *bool {
	return &value
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (s *PageService) storePageDocumentInput(input PageDocumentInput) (string, string, int64, string, string, error) {
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
			return "", "", 0, "", "", errors.New("document upload is missing both uploaded file and file_url")
		}
		if objectKey == "" {
			_, parsedObjectKey, err := util.ParseGCSObjectReference(strings.TrimSpace(s.BucketName), referenceURL)
			if err == nil {
				objectKey = s.relativeObjectKey(parsedObjectKey)
			}
		}
		return referenceURL, objectKey, 0, "", "", nil
	}

	if strings.TrimSpace(s.BucketName) == "" {
		return "", "", 0, "", "", ErrMediaBucketNotConfigured
	}

	objectName := s.pageDocumentObjectName(input.FileName, input.MimeType)
	storageObjectName := s.storageObjectName(objectName)

	var (
		fileURL string
		size    int64
		err     error
	)
	if len(input.Content) > 0 {
		fileURL, size, err = uploadBytesToGCSHook(input.Content, s.BucketName, storageObjectName, strings.TrimSpace(input.MimeType))
	} else {
		fileURL, size, err = uploadBase64ToGCSHook(input.DataBase64, s.BucketName, storageObjectName, strings.TrimSpace(input.MimeType))
	}
	if err != nil {
		return "", "", 0, "", "", err
	}

	checksum := ""
	if len(input.Content) > 0 {
		sum := sha256.Sum256(input.Content)
		checksum = hex.EncodeToString(sum[:])
	}

	return fileURL, objectName, size, checksum, storageObjectName, nil
}

func resolvePageDocumentOriginalFileName(input PageDocumentInput, existing PageDocument) string {
	if strings.TrimSpace(input.OriginalFileName) != "" {
		return strings.TrimSpace(input.OriginalFileName)
	}
	if strings.TrimSpace(input.FileName) != "" {
		return strings.TrimSpace(input.FileName)
	}
	if strings.TrimSpace(existing.OriginalFileName) != "" {
		return strings.TrimSpace(existing.OriginalFileName)
	}
	return buildPageDocumentFileName(
		input.DisplayName,
		"",
		firstNonBlank(input.ObjectKey, existing.GCPObjectKey),
		firstNonBlank(input.FileURL, input.StorageURI, existing.FileURL),
		firstNonBlank(input.MimeType, existing.MimeType),
	)
}

func (s *PageService) deleteOrphanPageDocument(tx *gorm.DB, documentID int) (*pageStoredObject, error) {
	var count int64
	if err := tx.Model(&PageSectionDocument{}).
		Where("document_id = ?", documentID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil
	}

	var row PageDocument
	if err := tx.First(&row, documentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if err := tx.Delete(&row).Error; err != nil {
		return nil, err
	}

	if strings.TrimSpace(row.FileURL) == "" && strings.TrimSpace(row.GCPObjectKey) == "" {
		return nil, nil
	}

	return &pageStoredObject{
		ObjectKey:  row.GCPObjectKey,
		StorageURL: row.FileURL,
	}, nil
}

func (s *PageService) cleanupStoredObjects(items []pageStoredObject) error {
	var cleanupErr error
	for _, item := range items {
		if err := s.cleanupStoredObject(item); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func (s *PageService) cleanupStoredObject(item pageStoredObject) error {
	bucketName, objectName, err := s.resolveStoredObjectReference(item)
	if err != nil {
		return err
	}
	if objectName == "" {
		return nil
	}
	return deleteGCSObjectHook(bucketName, objectName)
}

func (s *PageService) resolveStoredObjectReference(item pageStoredObject) (string, string, error) {
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

func shouldCleanupStoredObject(oldObjectKey string, oldURL string, newObjectKey string, newURL string) bool {
	return strings.TrimSpace(oldObjectKey) != strings.TrimSpace(newObjectKey) ||
		strings.TrimSpace(oldURL) != strings.TrimSpace(newURL)
}

func (s *PageService) pageDocumentObjectName(fileName string, mimeType string) string {
	timestamp := fmt.Sprintf("%d", pagesNowFunc().UTC().UnixNano())
	base := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	base = util.SanitizePart(base)
	if base == "unknown" {
		base = "document"
	}
	ext := util.ExtFromFilenameOrMime(fileName, mimeType)
	return fmt.Sprintf("page-documents/%s_%s%s", timestamp, base, ext)
}

func buildPageDocumentFetchURL(documentID int) string {
	return fmt.Sprintf("/api/pages/documents/%d/content", documentID)
}

func buildPageDocumentFileName(displayName string, originalFileName string, objectKey string, storageURL string, mimeType string) string {
	if trimmed := strings.TrimSpace(originalFileName); trimmed != "" {
		return trimmed
	}

	ext := path.Ext(strings.TrimSpace(objectKey))
	if ext == "" && strings.TrimSpace(storageURL) != "" {
		if _, parsedObjectKey, err := util.ParseGCSObjectReference("", storageURL); err == nil {
			ext = path.Ext(parsedObjectKey)
		}
	}
	if ext == "" {
		ext = util.ExtFromFilenameOrMime("", mimeType)
	}

	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		if path.Ext(trimmed) == "" && ext != "" {
			return trimmed + ext
		}
		return trimmed
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

	return "page-document" + ext
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *PageService) GetPageDocumentContent(id int) (*PageDocumentContent, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	var row PageDocument
	if err := s.DB.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageDocumentNotFound
		}
		return nil, err
	}

	content, contentType, err := s.downloadStoredObject(pageStoredObject{
		ObjectKey:  row.GCPObjectKey,
		StorageURL: row.FileURL,
	})
	if err != nil {
		if errors.Is(err, util.ErrObjectNotFound) {
			return nil, ErrPageDocumentNotFound
		}
		return nil, err
	}

	return &PageDocumentContent{
		Content:     content,
		ContentType: contentType,
		FileName: buildPageDocumentFileName(
			row.DisplayName,
			row.OriginalFileName,
			row.GCPObjectKey,
			row.FileURL,
			row.MimeType,
		),
	}, nil
}

func (s *PageService) downloadStoredObject(item pageStoredObject) ([]byte, string, error) {
	bucketName, objectName, err := s.resolveStoredObjectReference(item)
	if err != nil {
		return nil, "", err
	}
	return downloadGCSObjectHook(bucketName, objectName)
}

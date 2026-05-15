package pages

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	PageSectionTypeHeader     = "header"
	PageSectionTypeTypography = "typography"
	PageSectionTypeGallery    = "gallery"
	PageSectionTypeDocument   = "document"
	PageSectionTypeQuote      = "quote"
	PageSectionTypeCTABanner  = "cta_banner"

	PageHeaderHierarchyHero    = "h1_hero"
	PageHeaderHierarchySection = "h2_section"

	PageGalleryViewGrid     = "grid"
	PageGalleryViewCarousel = "carousel"
	PageGalleryViewMasonry  = "masonry"
	PageGalleryViewFocus    = "focus"

	PageTypographyAlignLeft   = "left"
	PageTypographyAlignCenter = "center"
	PageTypographyAlignRight  = "right"
)

type JSONRawMessage json.RawMessage

func (r JSONRawMessage) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return []byte(r), nil
}

func (r *JSONRawMessage) UnmarshalJSON(data []byte) error {
	if r == nil {
		return fmt.Errorf("json raw message target cannot be nil")
	}
	if data == nil {
		*r = nil
		return nil
	}

	*r = append((*r)[:0], data...)
	return nil
}

func (r JSONRawMessage) Value() (driver.Value, error) {
	if len(r) == 0 {
		return nil, nil
	}
	return []byte(r), nil
}

func (r *JSONRawMessage) Scan(value any) error {
	if value == nil {
		*r = nil
		return nil
	}

	switch typed := value.(type) {
	case []byte:
		*r = append((*r)[:0], typed...)
		return nil
	case string:
		*r = append((*r)[:0], typed...)
		return nil
	default:
		return fmt.Errorf("unsupported json raw message scan type %T", value)
	}
}

type PageContentDetail struct {
	ID            int            `gorm:"primaryKey;autoIncrement" json:"id"`
	PageID        int            `gorm:"not null;uniqueIndex;column:page_id" json:"page_id"`
	TemplateKey   string         `gorm:"size:100;not null;default:default;column:template_key" json:"template_key"`
	Settings      JSONRawMessage `gorm:"type:jsonb;column:settings" json:"settings,omitempty"`
	SchemaVersion int            `gorm:"not null;default:1;column:schema_version" json:"schema_version"`
	CreatedBy     *int           `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy     *int           `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type PageSection struct {
	ID           int            `gorm:"primaryKey;autoIncrement" json:"id"`
	PageDetailID int            `gorm:"not null;column:page_detail_id" json:"page_detail_id"`
	SectionName  string         `gorm:"size:150;not null;column:section_name" json:"section_name"`
	SectionType  string         `gorm:"size:50;not null;column:section_type" json:"section_type"`
	SortOrder    int            `gorm:"not null;column:sort_order" json:"sort_order"`
	IsEnabled    bool           `gorm:"not null;default:true;column:is_enabled" json:"is_enabled"`
	Settings     JSONRawMessage `gorm:"type:jsonb;column:settings" json:"settings,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type PageSectionHeaderModule struct {
	PageSectionID  int       `gorm:"primaryKey;column:page_section_id" json:"page_section_id"`
	MainHeaderText string    `gorm:"size:255;column:main_header_text" json:"main_header_text"`
	SubHeaderText  string    `gorm:"size:255;column:sub_header_text" json:"sub_header_text"`
	Hierarchy      string    `gorm:"size:20;column:hierarchy" json:"hierarchy"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type PageSectionTypographyModule struct {
	PageSectionID int       `gorm:"primaryKey;column:page_section_id" json:"page_section_id"`
	BodyHTML      string    `gorm:"column:body_html" json:"body_html"`
	BodyText      string    `gorm:"column:body_text" json:"body_text"`
	TextAlign     string    `gorm:"size:20;column:text_align" json:"text_align"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PageSectionGalleryModule struct {
	PageSectionID int       `gorm:"primaryKey;column:page_section_id" json:"page_section_id"`
	GalleryID     *int      `gorm:"column:gallery_id" json:"gallery_id,omitempty"`
	ViewMode      string    `gorm:"size:20;column:view_mode" json:"view_mode"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PageSectionQuoteModule struct {
	PageSectionID int       `gorm:"primaryKey;column:page_section_id" json:"page_section_id"`
	QuoteContent  string    `gorm:"column:quote_content" json:"quote_content"`
	Attribution   string    `gorm:"size:255;column:attribution" json:"attribution"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PageSectionCTABannerModule struct {
	PageSectionID int       `gorm:"primaryKey;column:page_section_id" json:"page_section_id"`
	BannerHeading string    `gorm:"size:255;column:banner_heading" json:"banner_heading"`
	BannerMessage string    `gorm:"size:255;column:banner_message" json:"banner_message"`
	ButtonText    string    `gorm:"size:100;column:button_text" json:"button_text"`
	ButtonURL     string    `gorm:"column:button_url" json:"button_url"`
	OpenInNewTab  bool      `gorm:"not null;default:false;column:open_in_new_tab" json:"open_in_new_tab"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PageDocument struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	DisplayName      string    `gorm:"size:255;not null;column:display_name" json:"display_name"`
	Description      string    `gorm:"column:description" json:"description"`
	OriginalFileName string    `gorm:"size:255;column:original_file_name" json:"original_file_name"`
	GCPObjectKey     string    `gorm:"column:gcp_object_key" json:"gcp_object_key"`
	FileURL          string    `gorm:"not null;column:file_url" json:"file_url"`
	MimeType         string    `gorm:"size:255;column:mime_type" json:"mime_type"`
	FileSize         int64     `gorm:"column:file_size" json:"file_size"`
	ChecksumSHA256   *string   `gorm:"size:64;column:checksum_sha256" json:"checksum_sha256"`
	CreatedBy        *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy        *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PageSectionDocument struct {
	ID                  int       `gorm:"primaryKey;autoIncrement" json:"id"`
	PageSectionID       int       `gorm:"not null;column:page_section_id" json:"page_section_id"`
	DocumentID          int       `gorm:"not null;column:document_id" json:"document_id"`
	DisplayNameOverride string    `gorm:"size:255;column:display_name_override" json:"display_name_override"`
	SortOrder           int       `gorm:"not null;column:sort_order" json:"sort_order"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type PageHeaderSectionInput struct {
	MainHeaderText string `json:"main_header_text"`
	SubHeaderText  string `json:"sub_header_text"`
	Hierarchy      string `json:"hierarchy"`
}

type PageTypographySectionInput struct {
	HTMLContent string `json:"html_content"`
	TextContent string `json:"text_content"`
	TextAlign   string `json:"text_align"`
}

type PageGallerySectionInput struct {
	GalleryID *int   `json:"gallery_id"`
	ViewMode  string `json:"view_mode"`
}

type PageQuoteSectionInput struct {
	QuoteContent string `json:"quote_content"`
	Attribution  string `json:"attribution"`
}

type PageCTABannerSectionInput struct {
	BannerHeading string `json:"banner_heading"`
	BannerMessage string `json:"banner_message"`
	ButtonText    string `json:"button_text"`
	ButtonURL     string `json:"button_url"`
	OpenInNewTab  bool   `json:"open_in_new_tab"`
}

type PageDocumentInput struct {
	ID               *int   `json:"id,omitempty"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	OriginalFileName string `json:"original_file_name"`
	FileName         string `json:"file_name"`
	MimeType         string `json:"mime_type"`
	DataBase64       string `json:"data_base64"`
	Content          []byte `json:"-"`
	FileURL          string `json:"file_url"`
	StorageURI       string `json:"storage_uri"`
	ObjectKey        string `json:"object_key"`
	GCPObjectKey     string `json:"gcp_object_key"`
}

type PageDocumentsSectionInput struct {
	Items []PageDocumentInput `json:"items"`
}

type SavePageSectionRequest struct {
	ID          *int                        `json:"id,omitempty"`
	SectionName string                      `json:"section_name"`
	SectionType string                      `json:"section_type"`
	SortOrder   int                         `json:"sort_order"`
	IsEnabled   bool                        `json:"is_enabled"`
	Settings    JSONRawMessage              `json:"settings,omitempty"`
	Header      *PageHeaderSectionInput     `json:"header,omitempty"`
	Typography  *PageTypographySectionInput `json:"typography,omitempty"`
	Gallery     *PageGallerySectionInput    `json:"gallery,omitempty"`
	Quote       *PageQuoteSectionInput      `json:"quote,omitempty"`
	CTABanner   *PageCTABannerSectionInput  `json:"cta_banner,omitempty"`
	Documents   *PageDocumentsSectionInput  `json:"documents,omitempty"`
}

type SavePageDetailRequest struct {
	TemplateKey string                   `json:"template_key"`
	Settings    JSONRawMessage           `json:"settings,omitempty"`
	Sections    []SavePageSectionRequest `json:"sections"`
}

type PageHeaderSectionResponse struct {
	MainHeaderText string `json:"main_header_text"`
	SubHeaderText  string `json:"sub_header_text"`
	Hierarchy      string `json:"hierarchy"`
}

type PageTypographySectionResponse struct {
	HTMLContent string `json:"html_content"`
	TextContent string `json:"text_content"`
	TextAlign   string `json:"text_align"`
}

type PageGallerySectionResponse struct {
	GalleryID *int   `json:"gallery_id,omitempty"`
	ViewMode  string `json:"view_mode"`
}

type PageQuoteSectionResponse struct {
	QuoteContent string `json:"quote_content"`
	Attribution  string `json:"attribution"`
}

type PageCTABannerSectionResponse struct {
	BannerHeading string `json:"banner_heading"`
	BannerMessage string `json:"banner_message"`
	ButtonText    string `json:"button_text"`
	ButtonURL     string `json:"button_url"`
	OpenInNewTab  bool   `json:"open_in_new_tab"`
}

type PageDocumentResponse struct {
	ID               int       `json:"id"`
	DisplayName      string    `json:"display_name"`
	Description      string    `json:"description"`
	OriginalFileName string    `json:"original_file_name"`
	FileName         string    `json:"file_name"`
	FileURL          string    `json:"file_url"`
	FetchURL         string    `json:"fetch_url"`
	StorageURI       string    `json:"storage_uri"`
	GCPObjectKey     string    `json:"gcp_object_key"`
	MimeType         string    `json:"mime_type"`
	FileSize         int64     `json:"file_size"`
	SortOrder        int       `json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PageDocumentsSectionResponse struct {
	Items []PageDocumentResponse `json:"items"`
}

type PageSectionResponse struct {
	ID          int                            `json:"id"`
	SectionName string                         `json:"section_name"`
	SectionType string                         `json:"section_type"`
	SortOrder   int                            `json:"sort_order"`
	IsEnabled   bool                           `json:"is_enabled"`
	Settings    JSONRawMessage                 `json:"settings,omitempty"`
	Header      *PageHeaderSectionResponse     `json:"header,omitempty"`
	Typography  *PageTypographySectionResponse `json:"typography,omitempty"`
	Gallery     *PageGallerySectionResponse    `json:"gallery,omitempty"`
	Quote       *PageQuoteSectionResponse      `json:"quote,omitempty"`
	CTABanner   *PageCTABannerSectionResponse  `json:"cta_banner,omitempty"`
	Documents   *PageDocumentsSectionResponse  `json:"documents,omitempty"`
	CreatedAt   time.Time                      `json:"created_at"`
	UpdatedAt   time.Time                      `json:"updated_at"`
}

type PageContentDetailResponse struct {
	ID            int                   `json:"id"`
	PageID        int                   `json:"page_id"`
	TemplateKey   string                `json:"template_key"`
	Settings      JSONRawMessage        `json:"settings,omitempty"`
	SchemaVersion int                   `json:"schema_version"`
	Sections      []PageSectionResponse `json:"sections"`
	CreatedBy     *int                  `json:"created_by,omitempty"`
	UpdatedBy     *int                  `json:"updated_by,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

type PageDocumentContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (PageContentDetail) TableName() string {
	return "page_details"
}

func (PageSection) TableName() string {
	return "page_sections"
}

func (PageSectionHeaderModule) TableName() string {
	return "page_section_header_modules"
}

func (PageSectionTypographyModule) TableName() string {
	return "page_section_typography_modules"
}

func (PageSectionGalleryModule) TableName() string {
	return "page_section_gallery_modules"
}

func (PageSectionQuoteModule) TableName() string {
	return "page_section_quote_modules"
}

func (PageSectionCTABannerModule) TableName() string {
	return "page_section_cta_banner_modules"
}

func (PageDocument) TableName() string {
	return "documents"
}

func (PageSectionDocument) TableName() string {
	return "page_section_documents"
}

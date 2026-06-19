package blogs

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	BlogSectionTypeHeading    = "heading"
	BlogSectionTypeImage      = "image"
	BlogSectionTypeTypography = "typography"
	BlogSectionTypeAction     = "action"
	BlogSectionTypeVideo      = "video"
	BlogSectionTypeAnimation  = "animation"

	BlogActionTypeLink  = "link"
	BlogActionTypeVideo = "video"

	BlogAnimationNavigationVertical   = "vertical"
	BlogAnimationNavigationHorizontal = "horizontal"

	BlogAnimationImagePositionLeft  = "left"
	BlogAnimationImagePositionRight = "right"
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

type Blog struct {
	ID                  int       `gorm:"primaryKey;autoIncrement" json:"id"`
	PublishDate         time.Time `gorm:"column:publish_date;not null" json:"publish_date"`
	Heading             string    `gorm:"size:255;not null;column:heading" json:"heading"`
	Description         string    `gorm:"column:description;not null" json:"description"`
	CoverImageURL       string    `gorm:"column:cover_image_url" json:"cover_image_url"`
	CoverImageObjectKey string    `gorm:"column:cover_image_object_key" json:"cover_image_object_key"`
	CreatedBy           *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy           *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type BlogContentDetail struct {
	ID            int            `gorm:"primaryKey;autoIncrement" json:"id"`
	BlogID        int            `gorm:"not null;uniqueIndex;column:blog_id" json:"blog_id"`
	Settings      JSONRawMessage `gorm:"type:jsonb;column:settings" json:"settings,omitempty"`
	SchemaVersion int            `gorm:"not null;default:1;column:schema_version" json:"schema_version"`
	CreatedBy     *int           `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy     *int           `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type BlogSection struct {
	ID           int            `gorm:"primaryKey;autoIncrement" json:"id"`
	BlogDetailID int            `gorm:"not null;column:blog_detail_id" json:"blog_detail_id"`
	SectionName  string         `gorm:"size:150;not null;column:section_name" json:"section_name"`
	SectionType  string         `gorm:"size:50;not null;column:section_type" json:"section_type"`
	SortOrder    int            `gorm:"not null;column:sort_order" json:"sort_order"`
	IsEnabled    bool           `gorm:"not null;default:true;column:is_enabled" json:"is_enabled"`
	Settings     JSONRawMessage `gorm:"type:jsonb;column:settings" json:"settings,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type BlogSectionHeadingModule struct {
	BlogSectionID    int       `gorm:"primaryKey;column:blog_section_id" json:"blog_section_id"`
	HeadingText      string    `gorm:"size:255;not null;column:heading_text" json:"heading_text"`
	UnderlineEnabled bool      `gorm:"not null;default:false;column:underline_enabled" json:"underline_enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BlogSectionImageModule struct {
	BlogSectionID  int       `gorm:"primaryKey;column:blog_section_id" json:"blog_section_id"`
	ImageURL       string    `gorm:"column:image_url" json:"image_url"`
	ImageObjectKey string    `gorm:"column:image_object_key" json:"image_object_key"`
	Caption        string    `gorm:"column:caption" json:"caption"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BlogSectionTypographyModule struct {
	BlogSectionID int       `gorm:"primaryKey;column:blog_section_id" json:"blog_section_id"`
	BodyHTML      string    `gorm:"column:body_html" json:"body_html"`
	BodyText      string    `gorm:"column:body_text" json:"body_text"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type BlogSectionActionModule struct {
	BlogSectionID int       `gorm:"primaryKey;column:blog_section_id" json:"blog_section_id"`
	ActionText    string    `gorm:"size:255;not null;column:action_text" json:"action_text"`
	ActionType    string    `gorm:"size:20;not null;column:action_type" json:"action_type"`
	TargetURL     string    `gorm:"not null;column:target_url" json:"target_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type BlogSectionVideoModule struct {
	BlogSectionID int       `gorm:"primaryKey;column:blog_section_id" json:"blog_section_id"`
	YouTubeURL    string    `gorm:"not null;column:youtube_url" json:"youtube_url"`
	Caption       string    `gorm:"column:caption" json:"caption"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type BlogSectionAnimationModule struct {
	BlogSectionID int       `gorm:"primaryKey;column:blog_section_id" json:"blog_section_id"`
	Navigation    string    `gorm:"size:20;not null;column:navigation" json:"navigation"`
	ImagePosition string    `gorm:"size:20;not null;column:image_position" json:"image_position"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type BlogAnimationItem struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	BlogSectionID  int       `gorm:"not null;column:blog_section_id" json:"blog_section_id"`
	SortOrder      int       `gorm:"not null;column:sort_order" json:"sort_order"`
	Heading        string    `gorm:"size:255;not null;column:heading" json:"heading"`
	SubHeading     string    `gorm:"size:255;not null;column:sub_heading" json:"sub_heading"`
	Description    string    `gorm:"not null;column:description" json:"description"`
	ImageURL       string    `gorm:"column:image_url" json:"image_url"`
	ImageObjectKey string    `gorm:"column:image_object_key" json:"image_object_key"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BlogUploadInput struct {
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	DataBase64   string `json:"data_base64"`
	Content      []byte `json:"-"`
	FileURL      string `json:"file_url"`
	StorageURI   string `json:"storage_uri"`
	ObjectKey    string `json:"object_key"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SaveBlogRequest struct {
	PublishDate      string                 `json:"publish_date"`
	Heading          string                 `json:"heading"`
	Description      string                 `json:"description"`
	CoverImage       *BlogUploadInput       `json:"cover_image,omitempty"`
	RemoveCoverImage bool                   `json:"remove_cover_image"`
	BlogDetail       *SaveBlogDetailRequest `json:"blog_detail,omitempty"`
	CreatedBy        *int                   `json:"created_by,omitempty"`
	UpdatedBy        *int                   `json:"updated_by,omitempty"`
}

type BlogMutationResponse struct {
	ID          int       `json:"id"`
	PublishDate time.Time `json:"publish_date"`
	Heading     string    `json:"heading"`
}

type BlogListFilters struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	SearchTerm    string `json:"search_term"`
	SortBy        string `json:"sort_by"`
	SortOrder     string `json:"sort_order"`
	UsePagination bool   `json:"-"`
}

type BlogListItem struct {
	ID                  int       `json:"id"`
	PublishDate         time.Time `json:"publish_date"`
	Heading             string    `json:"heading"`
	Description         string    `json:"description"`
	CoverImageURL       string    `json:"cover_image_url"`
	CoverImageObjectKey string    `json:"cover_image_object_key"`
	CoverImageFetchURL  string    `json:"cover_image_fetch_url"`
	UpdatedBy           *int      `json:"updated_by,omitempty"`
	UpdatedByName       string    `json:"updated_by_name"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type BlogListPageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type BlogListResponse struct {
	Items      []BlogListItem   `json:"items"`
	Pagination BlogListPageMeta `json:"pagination"`
	Applied    BlogListFilters  `json:"applied_filters"`
}

type BlogHeadingSectionInput struct {
	HeadingText      string `json:"heading_text"`
	UnderlineEnabled *bool  `json:"underline_enabled,omitempty"`
}

type BlogImageSectionInput struct {
	Asset   *BlogUploadInput `json:"asset,omitempty"`
	Caption string           `json:"caption"`
}

type BlogTypographySectionInput struct {
	HTMLContent string `json:"html_content"`
	TextContent string `json:"text_content"`
}

type BlogActionSectionInput struct {
	Text       string `json:"text"`
	ActionType string `json:"action_type"`
	TargetURL  string `json:"target_url"`
}

type BlogVideoSectionInput struct {
	YouTubeURL string `json:"youtube_url"`
	Caption    string `json:"caption"`
}

type BlogAnimationItemInput struct {
	ID          *int             `json:"id,omitempty"`
	SortOrder   int              `json:"sort_order"`
	Heading     string           `json:"heading"`
	SubHeading  string           `json:"sub_heading"`
	Description string           `json:"description"`
	Image       *BlogUploadInput `json:"image,omitempty"`
}

type BlogAnimationSectionInput struct {
	Navigation    string                   `json:"navigation"`
	ImagePosition string                   `json:"image_position"`
	Items         []BlogAnimationItemInput `json:"items"`
}

type SaveBlogSectionRequest struct {
	ID          *int                        `json:"id,omitempty"`
	SectionName string                      `json:"section_name"`
	SectionType string                      `json:"section_type"`
	SortOrder   int                         `json:"sort_order"`
	IsEnabled   bool                        `json:"is_enabled"`
	Settings    JSONRawMessage              `json:"settings,omitempty"`
	Heading     *BlogHeadingSectionInput    `json:"heading,omitempty"`
	Image       *BlogImageSectionInput      `json:"image,omitempty"`
	Typography  *BlogTypographySectionInput `json:"typography,omitempty"`
	Action      *BlogActionSectionInput     `json:"action,omitempty"`
	Video       *BlogVideoSectionInput      `json:"video,omitempty"`
	Animation   *BlogAnimationSectionInput  `json:"animation,omitempty"`
}

type SaveBlogDetailRequest struct {
	Settings JSONRawMessage           `json:"settings,omitempty"`
	Sections []SaveBlogSectionRequest `json:"sections"`
}

type BlogSectionAssetResponse struct {
	FileURL      string `json:"file_url"`
	FetchURL     string `json:"fetch_url"`
	StorageURI   string `json:"storage_uri"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type BlogHeadingSectionResponse struct {
	HeadingText      string `json:"heading_text"`
	UnderlineEnabled bool   `json:"underline_enabled"`
}

type BlogImageSectionResponse struct {
	Asset   *BlogSectionAssetResponse `json:"asset,omitempty"`
	Caption string                    `json:"caption"`
}

type BlogTypographySectionResponse struct {
	HTMLContent string `json:"html_content"`
	TextContent string `json:"text_content"`
}

type BlogActionSectionResponse struct {
	Text       string `json:"text"`
	ActionType string `json:"action_type"`
	TargetURL  string `json:"target_url"`
}

type BlogVideoSectionResponse struct {
	YouTubeURL string `json:"youtube_url"`
	Caption    string `json:"caption"`
}

type BlogAnimationItemResponse struct {
	ID          int                       `json:"id"`
	SortOrder   int                       `json:"sort_order"`
	Heading     string                    `json:"heading"`
	SubHeading  string                    `json:"sub_heading"`
	Description string                    `json:"description"`
	Image       *BlogSectionAssetResponse `json:"image,omitempty"`
}

type BlogAnimationSectionResponse struct {
	Navigation    string                      `json:"navigation"`
	ImagePosition string                      `json:"image_position"`
	Items         []BlogAnimationItemResponse `json:"items"`
}

type BlogSectionResponse struct {
	ID          int                            `json:"id"`
	SectionName string                         `json:"section_name"`
	SectionType string                         `json:"section_type"`
	SortOrder   int                            `json:"sort_order"`
	IsEnabled   bool                           `json:"is_enabled"`
	Settings    JSONRawMessage                 `json:"settings,omitempty"`
	Heading     *BlogHeadingSectionResponse    `json:"heading,omitempty"`
	Image       *BlogImageSectionResponse      `json:"image,omitempty"`
	Typography  *BlogTypographySectionResponse `json:"typography,omitempty"`
	Action      *BlogActionSectionResponse     `json:"action,omitempty"`
	Video       *BlogVideoSectionResponse      `json:"video,omitempty"`
	Animation   *BlogAnimationSectionResponse  `json:"animation,omitempty"`
	CreatedAt   time.Time                      `json:"created_at"`
	UpdatedAt   time.Time                      `json:"updated_at"`
}

type BlogContentDetailResponse struct {
	ID            int                   `json:"id"`
	BlogID        int                   `json:"blog_id"`
	Settings      JSONRawMessage        `json:"settings,omitempty"`
	SchemaVersion int                   `json:"schema_version"`
	Sections      []BlogSectionResponse `json:"sections"`
	CreatedBy     *int                  `json:"created_by,omitempty"`
	UpdatedBy     *int                  `json:"updated_by,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

type BlogDetailResponse struct {
	ID                  int                        `json:"id"`
	PublishDate         time.Time                  `json:"publish_date"`
	Heading             string                     `json:"heading"`
	Description         string                     `json:"description"`
	CoverImageURL       string                     `json:"cover_image_url"`
	CoverImageObjectKey string                     `json:"cover_image_object_key"`
	CoverImageFetchURL  string                     `json:"cover_image_fetch_url"`
	CreatedBy           *int                       `json:"created_by,omitempty"`
	CreatedByName       string                     `json:"created_by_name"`
	UpdatedBy           *int                       `json:"updated_by,omitempty"`
	UpdatedByName       string                     `json:"updated_by_name"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
	BlogDetail          *BlogContentDetailResponse `json:"blog_detail,omitempty"`
}

type BlogMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (Blog) TableName() string {
	return "blogs"
}

func (BlogContentDetail) TableName() string        { return "blog_details" }
func (BlogSection) TableName() string              { return "blog_sections" }
func (BlogSectionHeadingModule) TableName() string { return "blog_section_heading_modules" }
func (BlogSectionImageModule) TableName() string   { return "blog_section_image_modules" }
func (BlogSectionTypographyModule) TableName() string {
	return "blog_section_typography_modules"
}
func (BlogSectionActionModule) TableName() string { return "blog_section_action_modules" }
func (BlogSectionVideoModule) TableName() string  { return "blog_section_video_modules" }
func (BlogSectionAnimationModule) TableName() string {
	return "blog_section_animation_modules"
}
func (BlogAnimationItem) TableName() string { return "blog_animation_items" }

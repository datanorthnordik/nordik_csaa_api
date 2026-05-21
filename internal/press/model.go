package press

import "time"

type PressEntry struct {
	ID               int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Title            string     `gorm:"size:255;not null" json:"title"`
	ReleaseDate      time.Time  `gorm:"column:release_date;not null" json:"release_date"`
	CategoryID       *int       `gorm:"column:category_id" json:"category_id,omitempty"`
	SourceURL        string     `gorm:"column:source_url" json:"source_url"`
	ContentHTML      string     `gorm:"column:content_html;type:text;not null;default:''" json:"content_html"`
	Status           string     `gorm:"size:30;not null;default:draft" json:"status"`
	Visibility       string     `gorm:"size:30;not null;default:private" json:"visibility"`
	CoverImageURL    string     `gorm:"column:cover_image_url" json:"cover_image_url,omitempty"`
	CoverImageGCPKey string     `gorm:"column:cover_image_gcp_key" json:"cover_image_gcp_key,omitempty"`
	PublishAt        *time.Time `gorm:"column:publish_at" json:"publish_at,omitempty"`
	CreatedBy        *int       `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy        *int       `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type PressMedia struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	PressEntryID int       `gorm:"not null;column:press_entry_id" json:"press_entry_id"`
	DisplayName  string    `gorm:"size:255;not null" json:"display_name"`
	FileName     string    `gorm:"size:255" json:"file_name"`
	GCPObjectKey string    `gorm:"column:gcp_object_key" json:"gcp_object_key,omitempty"`
	FileURL      string    `gorm:"not null;column:file_url" json:"file_url"`
	MimeType     string    `gorm:"size:255;column:mime_type" json:"mime_type"`
	FileSize     int64     `gorm:"column:file_size" json:"file_size,omitempty"`
	MediaRole    string    `gorm:"size:50;not null;default:attachment;column:media_role" json:"media_role"`
	SortOrder    int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	CreatedBy    *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy    *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Request/Response DTOs

type PressUploadInput struct {
	DisplayName  string `json:"display_name"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	Content      []byte `json:"-"`
	FileURL      string `json:"file_url"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SavePressEntryRequest struct {
	Title            string            `json:"title" binding:"required"`
	ReleaseDate      string            `json:"release_date" binding:"required"`
	CategoryID       *int              `json:"category_id,omitempty"`
	SourceURL        string            `json:"source_url"`
	ContentHTML      string            `json:"content_html"`
	Status           string            `json:"status" binding:"required"`
	Visibility       string            `json:"visibility" binding:"required"`
	PublishAt        *string           `json:"publish_at,omitempty"`
	CoverImage       *PressUploadInput `json:"cover_image,omitempty"`
	RemoveCoverImage bool              `json:"remove_cover_image,omitempty"`
}

type AddPressMediaRequest struct {
	Media []PressUploadInput `json:"media" binding:"required"`
}

type UpdatePressMediaRequest struct {
	DisplayName string `json:"display_name"`
	FileName    string `json:"file_name"`
}

type DeletePressMediaRequest struct {
	MediaIDs []int `json:"media_ids" binding:"required"`
}

type ReorderPressMediaRequest struct {
	MediaIDs []int `json:"media_ids" binding:"required"`
}

// Response DTOs

type PressMediaResponse struct {
	ID           int       `json:"id"`
	DisplayName  string    `json:"display_name"`
	FileName     string    `json:"file_name"`
	GCPObjectKey string    `json:"gcp_object_key,omitempty"`
	FileURL      string    `json:"file_url"`
	MimeType     string    `json:"mime_type"`
	FileSize     int64     `json:"file_size"`
	MediaRole    string    `json:"media_role"`
	SortOrder    int       `json:"sort_order"`
	CreatedBy    *int      `json:"created_by,omitempty"`
	UpdatedBy    *int      `json:"updated_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PressSummaryItem struct {
	ID               int                  `json:"id"`
	Title            string               `json:"title"`
	ReleaseDate      time.Time            `json:"release_date"`
	CategoryID       *int                 `json:"category_id,omitempty"`
	SourceURL        string               `json:"source_url"`
	ContentHTML      string               `json:"content_html"`
	Status           string               `json:"status"`
	Visibility       string               `json:"visibility"`
	CoverImageURL    string               `json:"cover_image_url,omitempty"`
	CoverImageGCPKey string               `json:"cover_image_gcp_key,omitempty"`
	PublishAt        *time.Time           `json:"publish_at,omitempty"`
	Media            []PressMediaResponse `json:"media"`
	CreatedBy        *int                 `json:"created_by,omitempty"`
	UpdatedBy        *int                 `json:"updated_by,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type PressListResponse struct {
	Items      []PressSummaryItem `json:"items"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int64              `json:"total_pages"`
}

type PressDetailResponse struct {
	ID               int                  `json:"id"`
	Title            string               `json:"title"`
	ReleaseDate      time.Time            `json:"release_date"`
	CategoryID       *int                 `json:"category_id,omitempty"`
	SourceURL        string               `json:"source_url"`
	ContentHTML      string               `json:"content_html"`
	Status           string               `json:"status"`
	Visibility       string               `json:"visibility"`
	CoverImageURL    string               `json:"cover_image_url,omitempty"`
	CoverImageGCPKey string               `json:"cover_image_gcp_key,omitempty"`
	PublishAt        *time.Time           `json:"publish_at,omitempty"`
	Media            []PressMediaResponse `json:"media"`
	CreatedBy        *int                 `json:"created_by,omitempty"`
	UpdatedBy        *int                 `json:"updated_by,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type PressMutationResponse struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	ReleaseDate time.Time `json:"release_date"`
	Status      string    `json:"status"`
	Visibility  string    `json:"visibility"`
}

type AddPressMediaResponse struct {
	UploadedCount int `json:"uploadedCount"`
}

type DeletePressMediaResponse struct {
	DeletedCount int `json:"deletedCount"`
}

type ReorderPressMediaResponse struct {
	UpdatedCount int `json:"updatedCount"`
}

type PressMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

// Table names

func (PressEntry) TableName() string {
	return "press_entries"
}

func (PressMedia) TableName() string {
	return "press_media"
}

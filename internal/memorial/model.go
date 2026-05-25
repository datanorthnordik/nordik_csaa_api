package memorial

import "time"

const (
	MemorialStatusDraft     = "draft"
	MemorialStatusReview    = "review"
	MemorialStatusPublished = "published"

	MemorialCategoryAlumnus = "alumnus"
	MemorialCategoryVeteran = "veteran"
	MemorialCategoryFounder = "founder"
	MemorialCategoryFriend  = "friend"
)

type MemorialEntry struct {
	ID                   int        `gorm:"primaryKey;autoIncrement" json:"id"`
	FullName             string     `gorm:"size:255;not null;column:full_name" json:"full_name"`
	Affiliation          string     `gorm:"size:255;column:affiliation" json:"affiliation"`
	Category             string     `gorm:"size:50;not null;column:category" json:"category"`
	Status               string     `gorm:"size:20;not null;default:draft;column:status" json:"status"`
	Biography            string     `gorm:"type:text;column:biography" json:"biography"`
	DateOfBirth          *time.Time `gorm:"type:date;column:date_of_birth" json:"date_of_birth,omitempty"`
	DateOfPassing        *time.Time `gorm:"type:date;column:date_of_passing" json:"date_of_passing,omitempty"`
	PublishedAt          *time.Time `gorm:"column:published_at" json:"published_at,omitempty"`
	PortraitFileName     string     `gorm:"size:255;column:portrait_file_name" json:"portrait_file_name,omitempty"`
	PortraitGCPObjectKey string     `gorm:"column:portrait_gcp_object_key" json:"portrait_gcp_object_key,omitempty"`
	PortraitFileURL      string     `gorm:"column:portrait_file_url" json:"portrait_file_url,omitempty"`
	PortraitMimeType     string     `gorm:"size:255;column:portrait_mime_type" json:"portrait_mime_type,omitempty"`
	PortraitFileSize     int64      `gorm:"column:portrait_file_size" json:"portrait_file_size,omitempty"`
	CreatedBy            *int       `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy            *int       `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type MemorialGalleryImage struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	MemorialEntryID int       `gorm:"not null;column:memorial_entry_id" json:"memorial_entry_id"`
	FileName        string    `gorm:"size:255;not null;column:file_name" json:"file_name"`
	GCPObjectKey    string    `gorm:"column:gcp_object_key" json:"gcp_object_key,omitempty"`
	FileURL         string    `gorm:"not null;column:file_url" json:"file_url"`
	MimeType        string    `gorm:"size:255;column:mime_type" json:"mime_type"`
	FileSize        int64     `gorm:"column:file_size" json:"file_size,omitempty"`
	SortOrder       int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	UploadedBy      *int      `gorm:"column:uploaded_by" json:"uploaded_by,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MemorialUploadInput struct {
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	Content      []byte `json:"-"`
	FileURL      string `json:"file_url"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SaveMemorialRequest struct {
	FullName              string                `json:"full_name" binding:"required"`
	Affiliation           string                `json:"affiliation"`
	Category              string                `json:"category" binding:"required"`
	Status                string                `json:"status" binding:"required"`
	Biography             string                `json:"biography"`
	DateOfBirth           string                `json:"date_of_birth"`
	DateOfPassing         string                `json:"date_of_passing"`
	Portrait              *MemorialUploadInput  `json:"portrait,omitempty"`
	RemovePortrait        bool                  `json:"remove_portrait"`
	GalleryImages         []MemorialUploadInput `json:"gallery_images,omitempty"`
	RemoveGalleryImageIDs []int                 `json:"remove_gallery_image_ids,omitempty"`
}

type MemorialMutationResponse struct {
	ID        int       `json:"id"`
	FullName  string    `json:"full_name"`
	Category  string    `json:"category"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MemorialMediaResponse struct {
	FileName   string    `json:"file_name"`
	MimeType   string    `json:"mime_type"`
	FileSize   int64     `json:"file_size"`
	ContentURL string    `json:"content_url"`
	CreatedAt  time.Time `json:"created_at"`
}

type MemorialGalleryImageResponse struct {
	ID         int       `json:"id"`
	FileName   string    `json:"file_name"`
	MimeType   string    `json:"mime_type"`
	FileSize   int64     `json:"file_size"`
	SortOrder  int       `json:"sort_order"`
	ContentURL string    `json:"content_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MemorialListItem struct {
	ID                 int        `json:"id"`
	FullName           string     `json:"full_name"`
	Affiliation        string     `json:"affiliation"`
	Category           string     `json:"category"`
	CategoryLabel      string     `json:"category_label"`
	Status             string     `json:"status"`
	DateOfBirth        string     `json:"date_of_birth,omitempty"`
	DateOfPassing      string     `json:"date_of_passing,omitempty"`
	PortraitContentURL string     `json:"portrait_content_url,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
}

type MemorialDetailResponse struct {
	ID            int                            `json:"id"`
	FullName      string                         `json:"full_name"`
	Affiliation   string                         `json:"affiliation"`
	Category      string                         `json:"category"`
	CategoryLabel string                         `json:"category_label"`
	Status        string                         `json:"status"`
	Biography     string                         `json:"biography"`
	DateOfBirth   string                         `json:"date_of_birth,omitempty"`
	DateOfPassing string                         `json:"date_of_passing,omitempty"`
	Portrait      *MemorialMediaResponse         `json:"portrait,omitempty"`
	GalleryImages []MemorialGalleryImageResponse `json:"gallery_images"`
	CreatedAt     time.Time                      `json:"created_at"`
	UpdatedAt     time.Time                      `json:"updated_at"`
	PublishedAt   *time.Time                     `json:"published_at,omitempty"`
}

type MemorialListPageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type MemorialListAppliedFilters struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	SearchTerm string `json:"search_term"`
	Status     string `json:"status"`
	Category   string `json:"category"`
}

type MemorialCategoryCount struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Count    int64  `json:"count"`
}

type MemorialStatusCount struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Count  int64  `json:"count"`
}

type MemorialListSummary struct {
	CategoryCounts []MemorialCategoryCount `json:"category_counts"`
	StatusCounts   []MemorialStatusCount   `json:"status_counts"`
}

type MemorialListResponse struct {
	Items      []MemorialListItem         `json:"items"`
	Pagination MemorialListPageMeta       `json:"pagination"`
	Summary    MemorialListSummary        `json:"summary"`
	Applied    MemorialListAppliedFilters `json:"applied_filters"`
}

type MemorialMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (MemorialEntry) TableName() string {
	return "memorial_entries"
}

func (MemorialGalleryImage) TableName() string {
	return "memorial_gallery_images"
}

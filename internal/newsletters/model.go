package newsletters

import "time"

type NewsletterEntry struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string     `gorm:"size:255;not null" json:"title"`
	Category    string     `gorm:"size:20;not null;default:''" json:"category"`
	SendDate    time.Time  `gorm:"column:send_date;not null" json:"send_date"`
	ContentHTML string     `gorm:"column:content_html;type:text;not null;default:''" json:"content_html"`
	Status      string     `gorm:"size:30;not null;default:draft" json:"status"`
	Visibility  string     `gorm:"size:30;not null;default:public" json:"visibility"`
	PublishAt   *time.Time `gorm:"column:publish_at" json:"publish_at,omitempty"`
	CreatedBy   *int       `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy   *int       `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type NewsletterMedia struct {
	ID                int       `gorm:"primaryKey;autoIncrement" json:"id"`
	NewsletterEntryID int       `gorm:"not null;column:newsletter_entry_id" json:"newsletter_entry_id"`
	DisplayName       string    `gorm:"size:255;not null" json:"display_name"`
	FileName          string    `gorm:"size:255" json:"file_name"`
	GCPObjectKey      string    `gorm:"column:gcp_object_key" json:"gcp_object_key,omitempty"`
	FileURL           string    `gorm:"not null;column:file_url" json:"file_url"`
	MimeType          string    `gorm:"size:255;column:mime_type" json:"mime_type"`
	FileSize          int64     `gorm:"column:file_size" json:"file_size,omitempty"`
	MediaRole         string    `gorm:"size:50;not null;default:attachment;column:media_role" json:"media_role"`
	SortOrder         int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	CreatedBy         *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy         *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type NewsletterUploadInput struct {
	DisplayName  string `json:"display_name"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	Content      []byte `json:"-"`
	FileURL      string `json:"file_url"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SaveNewsletterEntryRequest struct {
	Title       string  `json:"title" binding:"required"`
	Category    string  `json:"category"`
	SendDate    string  `json:"send_date" binding:"required"`
	ContentHTML string  `json:"content_html"`
	Status      string  `json:"status" binding:"required"`
	Visibility  string  `json:"visibility" binding:"required"`
	PublishAt   *string `json:"publish_at,omitempty"`
}

type AddNewsletterMediaRequest struct {
	Media []NewsletterUploadInput `json:"media" binding:"required"`
}

type UpdateNewsletterMediaRequest struct {
	DisplayName string `json:"display_name"`
	FileName    string `json:"file_name"`
}

type DeleteNewsletterMediaRequest struct {
	MediaIDs []int `json:"media_ids" binding:"required"`
}

type ReorderNewsletterMediaRequest struct {
	MediaIDs []int `json:"media_ids" binding:"required"`
}

type NewsletterMediaResponse struct {
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

type NewsletterSummaryItem struct {
	ID         int       `json:"id"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	SendDate   time.Time `json:"send_date"`
	Status     string    `json:"status"`
	Visibility string    `json:"visibility"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type NewsletterListResponse struct {
	Items      []NewsletterSummaryItem `json:"items"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"page_size"`
	TotalPages int64                   `json:"total_pages"`
}

type NewsletterDetailResponse struct {
	ID          int                       `json:"id"`
	Title       string                    `json:"title"`
	Category    string                    `json:"category"`
	SendDate    time.Time                 `json:"send_date"`
	ContentHTML string                    `json:"content_html"`
	Status      string                    `json:"status"`
	Visibility  string                    `json:"visibility"`
	PublishAt   *time.Time                `json:"publish_at,omitempty"`
	Media       []NewsletterMediaResponse `json:"media"`
	CreatedBy   *int                      `json:"created_by,omitempty"`
	UpdatedBy   *int                      `json:"updated_by,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
}

type NewsletterMutationResponse struct {
	ID         int       `json:"id"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	SendDate   time.Time `json:"send_date"`
	Status     string    `json:"status"`
	Visibility string    `json:"visibility"`
}

type AddNewsletterMediaResponse struct {
	UploadedCount int `json:"uploadedCount"`
}

type DeleteNewsletterMediaResponse struct {
	DeletedCount int `json:"deletedCount"`
}

type ReorderNewsletterMediaResponse struct {
	UpdatedCount int `json:"updatedCount"`
}

type NewsletterMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (NewsletterEntry) TableName() string {
	return "newsletter_entries"
}

func (NewsletterMedia) TableName() string {
	return "newsletter_media"
}

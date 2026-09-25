package latestcontent

import "time"

const (
	SourceTypeEvent      = "event"
	SourceTypeNewsletter = "newsletter"
	SourceTypePress      = "press"
	MaxItems             = 10
)

type LatestContent struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceType  string    `gorm:"size:20;not null;column:source_type" json:"source_type"`
	SourceID    int       `gorm:"not null;column:source_id" json:"source_id"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text;not null;default:''" json:"description"`
	DisplayDate time.Time `gorm:"not null;column:display_date" json:"display_date"`
	PublishedAt time.Time `gorm:"not null;column:published_at" json:"published_at"`
	CreatedAt   time.Time `gorm:"not null;column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;column:updated_at" json:"updated_at"`
}

func (LatestContent) TableName() string {
	return "latest_content"
}

type LatestContentItem struct {
	ID          int       `json:"id"`
	SourceType  string    `json:"source_type"`
	SourceID    int       `json:"source_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DisplayDate time.Time `json:"display_date"`
	PublishedAt time.Time `json:"published_at"`
	DetailPath  string    `json:"detail_path"`
}

type LatestContentResponse struct {
	Items []LatestContentItem `json:"items"`
}

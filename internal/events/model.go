package events

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

const (
	MediaRoleDisplayImage = "display_image"
	MediaRoleAttachment   = "attachment"
)

type Event struct {
	ID                          int            `gorm:"primaryKey;autoIncrement" json:"id"`
	Title                       string         `gorm:"size:255;not null" json:"title"`
	ShowTitle                   bool           `gorm:"not null;default:true;column:show_title" json:"show_title"`
	Categories                  pq.StringArray `gorm:"type:text[];not null;default:'{}'" json:"categories"`
	EventType                   string         `gorm:"size:30;not null;column:event_type" json:"event_type"`
	StartAt                     time.Time      `gorm:"not null;column:start_at" json:"start_at"`
	EndAt                       *time.Time     `gorm:"column:end_at" json:"end_at,omitempty"`
	PrivacyType                 string         `gorm:"size:20;not null;default:public;column:privacy_type" json:"privacy_type"`
	PrivateAudiences            pq.StringArray `gorm:"type:text[];not null;default:'{}';column:private_audiences" json:"private_audiences"`
	Published                   bool           `gorm:"not null;default:false" json:"published"`
	RequestReview               bool           `gorm:"not null;default:false;column:request_review" json:"request_review"`
	ReviewEmailList             pq.StringArray `gorm:"type:text[];not null;default:'{}';column:review_email_list" json:"review_email_list"`
	Teaser                      string         `gorm:"not null;default:''" json:"teaser"`
	DescriptionHTML             string         `gorm:"column:description_html" json:"description_html"`
	ContactName                 string         `gorm:"size:150;column:contact_name" json:"contact_name"`
	ContactEmail                string         `gorm:"size:255;column:contact_email" json:"contact_email"`
	ContactPhone                string         `gorm:"size:30;column:contact_phone" json:"contact_phone"`
	ContactExt                  string         `gorm:"size:20;column:contact_ext" json:"contact_ext"`
	ContactFax                  string         `gorm:"size:30;column:contact_fax" json:"contact_fax"`
	LocationMode                string         `gorm:"size:30;not null;default:none;column:location_mode" json:"location_mode"`
	AddressID                   *int           `gorm:"column:address_id" json:"address_id,omitempty"`
	ShowDisplayImageWhenViewing bool           `gorm:"not null;default:true;column:show_display_image_when_viewing" json:"show_display_image_when_viewing"`
	GalleryID                   *int           `gorm:"column:gallery_id" json:"gallery_id,omitempty"`
	RegistrationEnabled         bool           `gorm:"not null;default:false;column:registration_enabled" json:"registration_enabled"`
	RegistrationStartAt         *time.Time     `gorm:"column:registration_start_at" json:"registration_start_at,omitempty"`
	RegistrationEndAt           *time.Time     `gorm:"column:registration_end_at" json:"registration_end_at,omitempty"`
	RegistrationURL             string         `gorm:"column:registration_url" json:"registration_url"`
	RepeatEnabled               bool           `gorm:"not null;default:false;column:repeat_enabled" json:"repeat_enabled"`
	RecurrenceType              *string        `gorm:"size:20;column:recurrence_type" json:"recurrence_type,omitempty"`
	RecurrenceFrequency         *string        `gorm:"size:20;column:recurrence_frequency" json:"recurrence_frequency,omitempty"`
	RecurrenceInterval          int            `gorm:"not null;default:1;column:recurrence_interval" json:"recurrence_interval"`
	RecurrenceUntil             *time.Time     `gorm:"column:recurrence_until" json:"recurrence_until,omitempty"`
	RecurrenceRule              JSONRawMessage `gorm:"type:jsonb;column:recurrence_rule" json:"recurrence_rule,omitempty"`
	CreatedBy                   *int           `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt                   time.Time      `json:"created_at"`
	UpdatedAt                   time.Time      `json:"updated_at"`
}

type Address struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string    `gorm:"size:150" json:"name"`
	AddressLine1  string    `gorm:"size:255;column:address_line_1" json:"address_line_1"`
	AddressLine2  string    `gorm:"size:255;column:address_line_2" json:"address_line_2"`
	City          string    `gorm:"size:100" json:"city"`
	ProvinceState string    `gorm:"size:100;column:province_state" json:"province_state"`
	PostalCode    string    `gorm:"size:20;column:postal_code" json:"postal_code"`
	Country       string    `gorm:"size:100" json:"country"`
	IsSaved       bool      `gorm:"not null;default:false;column:is_saved" json:"is_saved"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Gallery struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:150;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type EventMedia struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID      int       `gorm:"not null;column:event_id" json:"event_id"`
	MediaRole    string    `gorm:"size:30;not null;column:media_role" json:"media_role"`
	DisplayName  string    `gorm:"size:255;column:display_name" json:"display_name"`
	GCPObjectKey string    `gorm:"column:gcp_object_key" json:"gcp_object_key"`
	FileURL      string    `gorm:"not null;column:file_url" json:"file_url"`
	StorageURI   string    `gorm:"-" json:"storage_uri,omitempty"`
	FetchURL     string    `gorm:"-" json:"fetch_url,omitempty"`
	MimeType     string    `gorm:"size:255;column:mime_type" json:"mime_type"`
	FileSize     int64     `gorm:"column:file_size" json:"file_size"`
	SortOrder    int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type EventOccurrence struct {
	ID                int        `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID           int        `gorm:"not null;column:event_id" json:"event_id"`
	OccurrenceStartAt time.Time  `gorm:"not null;column:occurrence_start_at" json:"occurrence_start_at"`
	OccurrenceEndAt   *time.Time `gorm:"column:occurrence_end_at" json:"occurrence_end_at,omitempty"`
	OccurrenceKind    string     `gorm:"size:20;not null;default:scheduled;column:occurrence_kind" json:"occurrence_kind"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type EventAddressInput struct {
	ID            *int   `json:"id"`
	Name          string `json:"name"`
	AddressLine1  string `json:"address_line_1"`
	AddressLine2  string `json:"address_line_2"`
	City          string `json:"city"`
	ProvinceState string `json:"province_state"`
	PostalCode    string `json:"postal_code"`
	Country       string `json:"country"`
	IsSaved       bool   `json:"is_saved"`
}

type EventUploadInput struct {
	DisplayName  string `json:"display_name"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	DataBase64   string `json:"data_base64"`
	FileURL      string `json:"file_url"`
	StorageURI   string `json:"storage_uri"`
	ObjectKey    string `json:"object_key"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type EventOccurrenceInput struct {
	OccurrenceStartAt time.Time  `json:"occurrence_start_at"`
	OccurrenceEndAt   *time.Time `json:"occurrence_end_at"`
	OccurrenceKind    string     `json:"occurrence_kind"`
}

type SaveEventRequest struct {
	Title                       string                 `json:"title" binding:"required"`
	ShowTitle                   bool                   `json:"show_title"`
	Categories                  []string               `json:"categories"`
	EventType                   string                 `json:"event_type" binding:"required"`
	StartAt                     time.Time              `json:"start_at" binding:"required"`
	EndAt                       *time.Time             `json:"end_at"`
	PrivacyType                 string                 `json:"privacy_type"`
	PrivateAudiences            []string               `json:"private_audiences"`
	Published                   bool                   `json:"published"`
	RequestReview               bool                   `json:"request_review"`
	ReviewEmailList             []string               `json:"review_email_list"`
	Teaser                      string                 `json:"teaser"`
	DescriptionHTML             string                 `json:"description_html"`
	ContactName                 string                 `json:"contact_name"`
	ContactEmail                string                 `json:"contact_email"`
	ContactPhone                string                 `json:"contact_phone"`
	ContactExt                  string                 `json:"contact_ext"`
	ContactFax                  string                 `json:"contact_fax"`
	LocationMode                string                 `json:"location_mode"`
	Address                     *EventAddressInput     `json:"address"`
	ShowDisplayImageWhenViewing bool                   `json:"show_display_image_when_viewing"`
	GalleryID                   *int                   `json:"gallery_id"`
	RegistrationEnabled         bool                   `json:"registration_enabled"`
	RegistrationStartAt         *time.Time             `json:"registration_start_at"`
	RegistrationEndAt           *time.Time             `json:"registration_end_at"`
	RegistrationURL             string                 `json:"registration_url"`
	RepeatEnabled               bool                   `json:"repeat_enabled"`
	RecurrenceType              string                 `json:"recurrence_type"`
	RecurrenceFrequency         string                 `json:"recurrence_frequency"`
	RecurrenceInterval          int                    `json:"recurrence_interval"`
	RecurrenceUntil             *time.Time             `json:"recurrence_until"`
	RecurrenceRule              json.RawMessage        `json:"recurrence_rule"`
	Occurrences                 []EventOccurrenceInput `json:"occurrences"`
	DisplayImage                *EventUploadInput      `json:"display_image"`
	Attachments                 []EventUploadInput     `json:"attachments"`
	CreatedBy                   *int                   `json:"created_by"`
}

type EventMutationResponse struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Published bool   `json:"published"`
}

type DeleteAllDocumentsResponse struct {
	DeletedCount int `json:"deletedCount"`
}

type SavedLocationListResponse struct {
	Items []Address `json:"items"`
}

type GalleryListResponse struct {
	Items []Gallery `json:"items"`
}

const (
	EventStatusPublished = "published"
	EventStatusDraft     = "draft"

	EventDateRangeCustom     = "custom"
	EventDateRangeNext30Days = "next_30_days"
	EventDateRangeLast30Days = "last_30_days"
	EventDateRangeToday      = "today"
	EventDateRangeThisMonth  = "this_month"
	EventDateRangeUpcoming   = "upcoming"
)

type ListEventsFilter struct {
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	SearchTerm string     `json:"search_term"`
	Statuses   []string   `json:"statuses"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	DateRange  string     `json:"date_range"`
	SortBy     string     `json:"sort_by"`
	SortOrder  string     `json:"sort_order"`
}

type EventListItem struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Categories  []string   `json:"categories"`
	Status      string     `json:"status"`
	Published   bool       `json:"published"`
	EventType   string     `json:"event_type"`
	StartAt     time.Time  `json:"start_at"`
	EndAt       *time.Time `json:"end_at,omitempty"`
	DateDisplay string     `json:"date_display"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type EventListResponse struct {
	Items      []EventListItem   `json:"items"`
	Pagination EventListPageMeta `json:"pagination"`
	Applied    ListEventsFilter  `json:"applied_filters"`
}

type EventDetailResponse struct {
	ID                          int               `json:"id"`
	Title                       string            `json:"title"`
	ShowTitle                   bool              `json:"show_title"`
	Categories                  []string          `json:"categories"`
	EventType                   string            `json:"event_type"`
	StartAt                     time.Time         `json:"start_at"`
	EndAt                       *time.Time        `json:"end_at,omitempty"`
	DateDisplay                 string            `json:"date_display"`
	PrivacyType                 string            `json:"privacy_type"`
	PrivateAudiences            []string          `json:"private_audiences"`
	Published                   bool              `json:"published"`
	RequestReview               bool              `json:"request_review"`
	ReviewEmailList             []string          `json:"review_email_list"`
	Teaser                      string            `json:"teaser"`
	DescriptionHTML             string            `json:"description_html"`
	ContactName                 string            `json:"contact_name"`
	ContactEmail                string            `json:"contact_email"`
	ContactPhone                string            `json:"contact_phone"`
	ContactExt                  string            `json:"contact_ext"`
	ContactFax                  string            `json:"contact_fax"`
	LocationMode                string            `json:"location_mode"`
	Address                     *Address          `json:"address,omitempty"`
	ShowDisplayImageWhenViewing bool              `json:"show_display_image_when_viewing"`
	GalleryID                   *int              `json:"gallery_id,omitempty"`
	RegistrationEnabled         bool              `json:"registration_enabled"`
	RegistrationStartAt         *time.Time        `json:"registration_start_at,omitempty"`
	RegistrationEndAt           *time.Time        `json:"registration_end_at,omitempty"`
	RegistrationURL             string            `json:"registration_url"`
	RepeatEnabled               bool              `json:"repeat_enabled"`
	RecurrenceType              *string           `json:"recurrence_type,omitempty"`
	RecurrenceFrequency         *string           `json:"recurrence_frequency,omitempty"`
	RecurrenceInterval          int               `json:"recurrence_interval"`
	RecurrenceUntil             *time.Time        `json:"recurrence_until,omitempty"`
	RecurrenceRule              JSONRawMessage    `json:"recurrence_rule,omitempty"`
	Occurrences                 []EventOccurrence `json:"occurrences"`
	DisplayImage                *EventMedia       `json:"display_image,omitempty"`
	Attachments                 []EventMedia      `json:"attachments"`
	CreatedBy                   *int              `json:"created_by,omitempty"`
	CreatedAt                   time.Time         `json:"created_at"`
	UpdatedAt                   time.Time         `json:"updated_at"`
}

type EventMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

type EventListPageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

func (Event) TableName() string {
	return "events"
}

func (Address) TableName() string {
	return "addresses"
}

func (Gallery) TableName() string {
	return "galleries"
}

func (EventMedia) TableName() string {
	return "event_media"
}

func (EventOccurrence) TableName() string {
	return "event_occurrences"
}

type JSONRawMessage json.RawMessage

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
	switch v := value.(type) {
	case []byte:
		*r = append((*r)[:0], v...)
		return nil
	case string:
		*r = append((*r)[:0], v...)
		return nil
	default:
		return fmt.Errorf("unsupported json raw message scan type %T", value)
	}
}

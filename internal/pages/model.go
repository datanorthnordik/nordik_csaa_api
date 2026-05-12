package pages

import "time"

const (
	PageStatusDraft     = "draft"
	PageStatusPublished = "published"
)

type Page struct {
	ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
	PageTitle          string    `gorm:"size:255;not null;column:page_title" json:"page_title"`
	URLSlug            string    `gorm:"size:255;not null;uniqueIndex;column:url_slug" json:"url_slug"`
	Status             string    `gorm:"size:20;not null;default:draft" json:"status"`
	HeroImageEnabled   bool      `gorm:"not null;default:false;column:hero_image_enabled" json:"hero_image_enabled"`
	HeroImageURL       string    `gorm:"column:hero_image_url" json:"hero_image_url"`
	HeroImageObjectKey string    `gorm:"column:hero_image_object_key" json:"hero_image_object_key"`
	SEOPageTitle       string    `gorm:"size:255;column:seo_page_title" json:"seo_page_title"`
	SEOPageDescription string    `gorm:"column:seo_page_description" json:"seo_page_description"`
	CreatedBy          *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	ModifiedBy         *int      `gorm:"column:modified_by" json:"modified_by,omitempty"`
	LastModified       time.Time `gorm:"autoUpdateTime;column:last_modified" json:"last_modified"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type PageUploadInput struct {
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	DataBase64   string `json:"data_base64"`
	FileURL      string `json:"file_url"`
	StorageURI   string `json:"storage_uri"`
	ObjectKey    string `json:"object_key"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SavePageRequest struct {
	PageTitle          string           `json:"page_title"`
	URLSlug            string           `json:"url_slug"`
	Status             string           `json:"status"`
	HeroImageEnabled   bool             `json:"hero_image_enabled"`
	HeroImage          *PageUploadInput `json:"hero_image"`
	RemoveHeroImage    bool             `json:"remove_hero_image"`
	SEOPageTitle       string           `json:"seo_page_title"`
	SEOPageDescription string           `json:"seo_page_description"`
	CreatedBy          *int             `json:"created_by"`
	ModifiedBy         *int             `json:"modified_by"`
}

type PageMutationResponse struct {
	ID        int    `json:"id"`
	PageTitle string `json:"page_title"`
	URLSlug   string `json:"url_slug"`
	Status    string `json:"status"`
}

type PageListFilters struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	SearchTerm string `json:"search_term"`
	Status     string `json:"status"`
	SortBy     string `json:"sort_by"`
	SortOrder  string `json:"sort_order"`
}

type PageListItem struct {
	ID             int       `json:"id"`
	PageTitle      string    `json:"page_title"`
	URLSlug        string    `json:"url_slug"`
	Status         string    `json:"status"`
	LastModified   time.Time `json:"last_modified"`
	ModifiedBy     *int      `json:"modified_by,omitempty"`
	ModifiedByName string    `json:"modified_by_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type PageListPageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type PageListResponse struct {
	Items      []PageListItem   `json:"items"`
	Pagination PageListPageMeta `json:"pagination"`
	Applied    PageListFilters  `json:"applied_filters"`
}

type PageDetailResponse struct {
	ID                 int       `json:"id"`
	PageTitle          string    `json:"page_title"`
	URLSlug            string    `json:"url_slug"`
	Status             string    `json:"status"`
	HeroImageEnabled   bool      `json:"hero_image_enabled"`
	HeroImageURL       string    `json:"hero_image_url"`
	HeroImageObjectKey string    `json:"hero_image_object_key"`
	HeroImageFetchURL  string    `json:"hero_image_fetch_url"`
	SEOPageTitle       string    `json:"seo_page_title"`
	SEOPageDescription string    `json:"seo_page_description"`
	CreatedBy          *int      `json:"created_by,omitempty"`
	CreatedByName      string    `json:"created_by_name"`
	ModifiedBy         *int      `json:"modified_by,omitempty"`
	ModifiedByName     string    `json:"modified_by_name"`
	LastModified       time.Time `json:"last_modified"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type PageHeroImageContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (Page) TableName() string {
	return "pages"
}

package resources

import "time"

const (
	ResourceCategoryBrandIdentity   = "brand_identity"
	ResourceCategoryGovernanceLegal = "governance_legal"
	ResourceCategoryTrainingManuals = "training_manuals"
	ResourceCategoryMediaKits       = "media_kits"

	ResourceVisibilityPublic   = "public"
	ResourceVisibilityInternal = "internal"
)

type ResourceEntry struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"size:255;not null" json:"name"`
	Category     string    `gorm:"size:50;not null" json:"category"`
	Visibility   string    `gorm:"size:20;not null;default:public" json:"visibility"`
	FileName     string    `gorm:"size:255;not null;column:file_name" json:"file_name"`
	GCPObjectKey string    `gorm:"column:gcp_object_key" json:"gcp_object_key,omitempty"`
	FileURL      string    `gorm:"not null;column:file_url" json:"file_url"`
	MimeType     string    `gorm:"size:255;column:mime_type" json:"mime_type"`
	FileSize     int64     `gorm:"column:file_size" json:"file_size,omitempty"`
	CreatedBy    *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy    *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ResourceUploadInput struct {
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	Content      []byte `json:"-"`
	FileURL      string `json:"file_url"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SaveResourceRequest struct {
	Name       string               `json:"name" binding:"required"`
	Category   string               `json:"category" binding:"required"`
	Visibility string               `json:"visibility" binding:"required"`
	Document   *ResourceUploadInput `json:"document,omitempty"`
}

type ResourceMutationResponse struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Visibility string    `json:"visibility"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ResourceCategoryCount struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Count    int64  `json:"count"`
}

type ResourceListItem struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	CategoryLabel string    `json:"category_label"`
	Visibility    string    `json:"visibility"`
	FileName      string    `json:"file_name"`
	MimeType      string    `json:"mime_type"`
	FileSize      int64     `json:"file_size"`
	ContentURL    string    `json:"content_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ResourceDetailResponse = ResourceListItem

type ResourceListPageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type ResourceListAppliedFilters struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	SearchTerm string `json:"search_term"`
	Category   string `json:"category"`
	FileType   string `json:"file_type"`
}

type ResourceListSummary struct {
	CategoryCounts []ResourceCategoryCount `json:"category_counts"`
}

type ResourceListResponse struct {
	Items      []ResourceListItem         `json:"items"`
	Pagination ResourceListPageMeta       `json:"pagination"`
	Summary    ResourceListSummary        `json:"summary"`
	Applied    ResourceListAppliedFilters `json:"applied_filters"`
}

type ResourceContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (ResourceEntry) TableName() string {
	return "resource_entries"
}

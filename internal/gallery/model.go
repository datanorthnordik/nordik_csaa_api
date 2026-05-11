package gallery

import "time"

type Gallery struct {
	ID                  int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                string    `gorm:"size:150;not null" json:"name"`
	Description         string    `gorm:"column:description" json:"description"`
	CoverImageURL       string    `gorm:"column:cover_image_url" json:"cover_image_url"`
	CoverImageObjectKey string    `gorm:"column:cover_image_object_key" json:"cover_image_object_key"`
	CoverImageAltText   string    `gorm:"size:255;column:cover_image_alt_text" json:"cover_image_alt_text"`
	Published           bool      `gorm:"not null;default:false" json:"published"`
	CreatedBy           *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy           *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type GalleryImage struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	GalleryID    int       `gorm:"not null;column:gallery_id" json:"gallery_id"`
	Title        string    `gorm:"size:255;column:title" json:"title"`
	AltText      string    `gorm:"size:255;column:alt_text" json:"alt_text"`
	GCPObjectKey string    `gorm:"column:gcp_object_key" json:"gcp_object_key"`
	FileURL      string    `gorm:"not null;column:file_url" json:"file_url"`
	MimeType     string    `gorm:"size:255;column:mime_type" json:"mime_type"`
	FileSize     int64     `gorm:"column:file_size" json:"file_size"`
	SortOrder    int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	UploadedBy   *int      `gorm:"column:uploaded_by" json:"uploaded_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type GalleryUploadInput struct {
	Title        string `json:"title"`
	AltText      string `json:"alt_text"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	DataBase64   string `json:"data_base64"`
	FileURL      string `json:"file_url"`
	StorageURI   string `json:"storage_uri"`
	ObjectKey    string `json:"object_key"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SaveGalleryRequest struct {
	Name             string              `json:"name" binding:"required"`
	Description      string              `json:"description"`
	Published        bool                `json:"published"`
	CoverImage       *GalleryUploadInput `json:"cover_image"`
	RemoveCoverImage bool                `json:"remove_cover_image"`
}

type AddGalleryImagesRequest struct {
	Images []GalleryUploadInput `json:"images" binding:"required"`
}

type UpdateGalleryImageRequest struct {
	Title   string `json:"title"`
	AltText string `json:"alt_text"`
}

type ReorderGalleryImagesRequest struct {
	ImageIDs []int `json:"image_ids" binding:"required"`
}

type DeleteGalleryImagesRequest struct {
	StorageURLs []string `json:"storage_urls" binding:"required"`
}

type GallerySummaryItem struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Published     bool      `json:"published"`
	AssetCount    int       `json:"asset_count"`
	FrontImageURL string    `json:"front_image_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GalleryListResponse struct {
	Items []GallerySummaryItem `json:"items"`
}

type GalleryAssetResponse struct {
	ID           int       `json:"id"`
	GalleryID    int       `json:"gallery_id"`
	Title        string    `json:"title"`
	AltText      string    `json:"alt_text"`
	FileName     string    `json:"file_name"`
	GCPObjectKey string    `json:"gcp_object_key,omitempty"`
	FileURL      string    `json:"file_url"`
	StorageURI   string    `json:"storage_uri,omitempty"`
	MimeType     string    `json:"mime_type"`
	FileSize     int64     `json:"file_size"`
	SortOrder    int       `json:"sort_order"`
	UploadedBy   *int      `json:"uploaded_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type GalleryDetailResponse struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Published   bool                   `json:"published"`
	AssetLimit  int                    `json:"asset_limit"`
	CoverImage  *GalleryAssetResponse  `json:"cover_image,omitempty"`
	Images      []GalleryAssetResponse `json:"images"`
	CreatedBy   *int                   `json:"created_by,omitempty"`
	UpdatedBy   *int                   `json:"updated_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type GalleryMutationResponse struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Published bool   `json:"published"`
}

type AddGalleryImagesResponse struct {
	UploadedCount int `json:"uploadedCount"`
}

type DeleteGalleryImagesResponse struct {
	DeletedCount int `json:"deletedCount"`
}

type ReorderGalleryImagesResponse struct {
	UpdatedCount int `json:"updatedCount"`
}

type GalleryMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (Gallery) TableName() string {
	return "galleries"
}

func (GalleryImage) TableName() string {
	return "gallery_images"
}

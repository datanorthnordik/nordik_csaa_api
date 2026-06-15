package video

import "time"

const (
	VideoPackageTypeSingle     = "single"
	VideoPackageTypeCollection = "collection"
)

type VideoPackage struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	PackageType string    `gorm:"size:20;not null;default:single;column:package_type" json:"package_type"`
	CreatedBy   *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy   *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type VideoItem struct {
	ID                   int       `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoPackageID       int       `gorm:"not null;column:video_package_id" json:"video_package_id"`
	Title                string    `gorm:"size:255;not null" json:"title"`
	YouTubeURL           string    `gorm:"column:youtube_url;not null" json:"youtube_url"`
	Description          string    `gorm:"column:description" json:"description"`
	TeaserImageURL       string    `gorm:"column:teaser_image_url;not null" json:"teaser_image_url"`
	TeaserImageObjectKey string    `gorm:"column:teaser_image_object_key" json:"teaser_image_object_key"`
	SortOrder            int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	CreatedBy            *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy            *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type VideoInput struct {
	Title             string `json:"title"`
	YouTubeURL        string `json:"youtube_url"`
	Description       string `json:"description"`
	FileName          string `json:"file_name"`
	MimeType          string `json:"mime_type"`
	DataBase64        string `json:"data_base64"`
	Content           []byte `json:"-"`
	FileURL           string `json:"file_url"`
	StorageURI        string `json:"storage_uri"`
	ObjectKey         string `json:"object_key"`
	GCPObjectKey      string `json:"gcp_object_key"`
	RemoveTeaserImage bool   `json:"remove_teaser_image,omitempty"`
}

type SaveVideoPackageRequest struct {
	Title       string       `json:"title"`
	PackageType string       `json:"package_type"`
	SingleVideo *VideoInput  `json:"single_video,omitempty"`
	Videos      []VideoInput `json:"videos,omitempty"`
}

type UpdateVideoPackageRequest struct {
	Title string `json:"title"`
}

type AddVideoItemsRequest struct {
	Videos []VideoInput `json:"videos" binding:"required"`
}

type UpdateVideoItemRequest = VideoInput

type VideoPackageSummaryItem struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	PackageType   string    `json:"package_type"`
	VideoCount    int       `json:"video_count"`
	FrontImageURL string    `json:"front_image_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type VideoPackageListResponse struct {
	Items []VideoPackageSummaryItem `json:"items"`
}

type VideoItemResponse struct {
	ID             int       `json:"id"`
	VideoPackageID int       `json:"video_package_id"`
	Title          string    `json:"title"`
	YouTubeURL     string    `json:"youtube_url"`
	Description    string    `json:"description"`
	TeaserImageURL string    `json:"teaser_image_url"`
	StorageURI     string    `json:"storage_uri,omitempty"`
	GCPObjectKey   string    `json:"gcp_object_key,omitempty"`
	SortOrder      int       `json:"sort_order"`
	CreatedBy      *int      `json:"created_by,omitempty"`
	UpdatedBy      *int      `json:"updated_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type VideoPackageDetailResponse struct {
	ID          int                 `json:"id"`
	Title       string              `json:"title"`
	PackageType string              `json:"package_type"`
	VideoCount  int                 `json:"video_count"`
	SingleVideo *VideoItemResponse  `json:"single_video,omitempty"`
	Videos      []VideoItemResponse `json:"videos"`
	CreatedBy   *int                `json:"created_by,omitempty"`
	UpdatedBy   *int                `json:"updated_by,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type VideoPackageMutationResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	PackageType string `json:"package_type"`
}

type AddVideoItemsResponse struct {
	UploadedCount int `json:"uploadedCount"`
}

type DeleteVideoItemResponse struct {
	DeletedCount int `json:"deletedCount"`
}

type VideoMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (VideoPackage) TableName() string {
	return "video_packages"
}

func (VideoItem) TableName() string {
	return "video_package_items"
}

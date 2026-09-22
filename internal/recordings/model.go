package recordings

import "time"

type RecordingCollection struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:150;not null;uniqueIndex" json:"name"`
	CreatedBy *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RecordingCollectionItem struct {
	ID                    int       `gorm:"primaryKey;autoIncrement" json:"id"`
	RecordingCollectionID int       `gorm:"not null;column:recording_collection_id" json:"recording_collection_id"`
	Title                 string    `gorm:"size:255;not null" json:"title"`
	Description           string    `gorm:"column:description" json:"description"`
	RecordingURL          string    `gorm:"column:recording_url" json:"recording_url"`
	RecordingObjectKey    string    `gorm:"column:recording_object_key" json:"recording_object_key"`
	SortOrder             int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	CreatedBy             *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy             *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// RecordingItemInput accepts an item either as a plain reference (file_url /
// storage_uri / object_key) or as an inline upload (content / data_base64). The
// recording itself is optional: an item may be created with a title and
// description only and have its recording attached later.
type RecordingItemInput struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	FileName        string `json:"file_name"`
	MimeType        string `json:"mime_type"`
	DataBase64      string `json:"data_base64"`
	Content         []byte `json:"-"`
	FileURL         string `json:"file_url"`
	StorageURI      string `json:"storage_uri"`
	RecordingURL    string `json:"recording_url"`
	ObjectKey       string `json:"object_key"`
	GCPObjectKey    string `json:"gcp_object_key"`
	RemoveRecording bool   `json:"remove_recording,omitempty"`
}

type SaveRecordingCollectionRequest struct {
	Name  string               `json:"name"`
	Items []RecordingItemInput `json:"items,omitempty"`
}

type UpdateRecordingCollectionRequest struct {
	Name string `json:"name"`
}

type AddRecordingItemsRequest struct {
	Items []RecordingItemInput `json:"items" binding:"required"`
}

type UpdateRecordingItemRequest = RecordingItemInput

type RecordingCollectionSummaryItem struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RecordingCollectionListResponse struct {
	Items []RecordingCollectionSummaryItem `json:"items"`
}

type RecordingItemResponse struct {
	ID                    int       `json:"id"`
	RecordingCollectionID int       `json:"recording_collection_id"`
	Title                 string    `json:"title"`
	Description           string    `json:"description"`
	RecordingURL          string    `json:"recording_url"`
	StorageURI            string    `json:"storage_uri,omitempty"`
	GCPObjectKey          string    `json:"gcp_object_key,omitempty"`
	SortOrder             int       `json:"sort_order"`
	CreatedBy             *int      `json:"created_by,omitempty"`
	UpdatedBy             *int      `json:"updated_by,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type RecordingCollectionDetailResponse struct {
	ID        int                     `json:"id"`
	Name      string                  `json:"name"`
	ItemCount int                     `json:"item_count"`
	Items     []RecordingItemResponse `json:"items"`
	CreatedBy *int                    `json:"created_by,omitempty"`
	UpdatedBy *int                    `json:"updated_by,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type RecordingCollectionMutationResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type AddRecordingItemsResponse struct {
	UploadedCount int `json:"uploadedCount"`
}

type DeleteRecordingItemResponse struct {
	DeletedCount int `json:"deletedCount"`
}

type RecordingMediaContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (RecordingCollection) TableName() string {
	return "recording_collections"
}

func (RecordingCollectionItem) TableName() string {
	return "recording_collection_items"
}

package recordings

// RecordingServicePort is the behaviour the HTTP layer depends on. Keeping the
// controller behind an interface matches the video/gallery modules and makes the
// service swappable in tests.
type RecordingServicePort interface {
	ListRecordingCollections() (*RecordingCollectionListResponse, error)
	GetRecordingCollection(id int) (*RecordingCollectionDetailResponse, error)
	GetRecordingItemContent(id int, itemID int) (*RecordingMediaContent, error)
	CreateRecordingCollection(req SaveRecordingCollectionRequest, userID *int) (*RecordingCollectionMutationResponse, error)
	UpdateRecordingCollection(id int, req UpdateRecordingCollectionRequest, userID *int) (*RecordingCollectionMutationResponse, error)
	DeleteRecordingCollection(id int) error
	AddRecordingItems(id int, req AddRecordingItemsRequest, userID *int) (*AddRecordingItemsResponse, error)
	UpdateRecordingItem(id int, itemID int, req UpdateRecordingItemRequest, userID *int) (*RecordingItemResponse, error)
	DeleteRecordingItem(id int, itemID int) (*DeleteRecordingItemResponse, error)
}

var _ RecordingServicePort = (*RecordingService)(nil)

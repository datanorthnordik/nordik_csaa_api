package events

type EventServicePort interface {
	ListEvents(filter ListEventsFilter) (*EventListResponse, error)
	GetEvent(id int) (*EventDetailResponse, error)
	GetEventMediaContent(eventID int, mediaID int) (*EventMediaContent, error)
	ListSavedLocations() (*SavedLocationListResponse, error)
	ListGalleries() (*GalleryListResponse, error)
	CreateEvent(req SaveEventRequest) (*EventMutationResponse, error)
	UpdateEvent(id int, req SaveEventRequest) (*EventMutationResponse, error)
	DeleteEvent(id int) error
	DeleteEventDocument(id int, storageURL string) error
	DeleteAllEventDocuments(id int, storageURLs []string) (*DeleteAllDocumentsResponse, error)
	DeleteEventPhoto(id int, storageURL string) error
}

var _ EventServicePort = (*EventService)(nil)

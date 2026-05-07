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
	DeleteEventDocument(id int, mediaID int) error
	DeleteAllEventDocuments(id int) (*DeleteAllDocumentsResponse, error)
	DeleteEventPhoto(id int, mediaID int) error
}

var _ EventServicePort = (*EventService)(nil)

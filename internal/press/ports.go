package press

type PressServicePort interface {
	ListPressEntries(filter ListPressFilter) (*PressListResponse, error)
	GetPressEntry(id int) (*PressDetailResponse, error)
	GetPressCoverImageContent(id int) (*PressMediaContent, error)
	GetPressMediaContent(id int, mediaID int) (*PressMediaContent, error)
	CreatePressEntry(req SavePressEntryRequest, userID *int) (*PressMutationResponse, error)
	UpdatePressEntry(id int, req SavePressEntryRequest, userID *int) (*PressMutationResponse, error)
	DeletePressEntry(id int) error
	AddPressMedia(id int, req AddPressMediaRequest, userID *int) (*AddPressMediaResponse, error)
	UpdatePressMedia(id int, mediaID int, req UpdatePressMediaRequest) (*PressMediaResponse, error)
	ReorderPressMedia(id int, mediaIDs []int) (*ReorderPressMediaResponse, error)
	DeletePressMedia(id int, mediaIDs []int) (*DeletePressMediaResponse, error)
}

type ListPressFilter struct {
	Status     string
	Visibility string
	SearchTerm string
	SortBy     string
	SortOrder  string
	Page       int
	PageSize   int
}

var _ PressServicePort = (*PressService)(nil)

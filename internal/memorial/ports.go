package memorial

type MemorialServicePort interface {
	ListMemorials(filter ListMemorialsFilter) (*MemorialListResponse, error)
	GetMemorial(id int) (*MemorialDetailResponse, error)
	GetMemorialPortraitContent(id int) (*MemorialMediaContent, error)
	GetMemorialGalleryImageContent(id int, mediaID int) (*MemorialMediaContent, error)
	CreateMemorial(req SaveMemorialRequest, userID *int) (*MemorialMutationResponse, error)
	UpdateMemorial(id int, req SaveMemorialRequest, userID *int) (*MemorialMutationResponse, error)
	DeleteMemorial(id int) error
}

type ListMemorialsFilter struct {
	Page       int
	PageSize   int
	SearchTerm string
	Status     string
	Category   string
}

var _ MemorialServicePort = (*MemorialService)(nil)

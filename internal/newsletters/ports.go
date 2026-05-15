package newsletters

type NewsletterServicePort interface {
	ListNewsletterEntries(filter ListNewsletterFilter) (*NewsletterListResponse, error)
	GetNewsletterEntry(id int) (*NewsletterDetailResponse, error)
	GetNewsletterMediaContent(id int, mediaID int) (*NewsletterMediaContent, error)
	CreateNewsletterEntry(req SaveNewsletterEntryRequest, userID *int) (*NewsletterMutationResponse, error)
	UpdateNewsletterEntry(id int, req SaveNewsletterEntryRequest, userID *int) (*NewsletterMutationResponse, error)
	DeleteNewsletterEntry(id int) error
	AddNewsletterMedia(id int, req AddNewsletterMediaRequest, userID *int) (*AddNewsletterMediaResponse, error)
	UpdateNewsletterMedia(id int, mediaID int, req UpdateNewsletterMediaRequest) (*NewsletterMediaResponse, error)
	ReorderNewsletterMedia(id int, mediaIDs []int) (*ReorderNewsletterMediaResponse, error)
	DeleteNewsletterMedia(id int, mediaIDs []int) (*DeleteNewsletterMediaResponse, error)
}

type ListNewsletterFilter struct {
	Status     string
	Visibility string
	SearchTerm string
	SortBy     string
	SortOrder  string
	Page       int
	PageSize   int
}

var _ NewsletterServicePort = (*NewsletterService)(nil)

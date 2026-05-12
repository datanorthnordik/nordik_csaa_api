package pages

type PageServicePort interface {
	ListPages(filter PageListFilters) (*PageListResponse, error)
	GetPage(id int) (*PageDetailResponse, error)
	GetPageHeroImageContent(id int) (*PageHeroImageContent, error)
	CreatePage(req SavePageRequest) (*PageMutationResponse, error)
	UpdatePage(id int, req SavePageRequest) (*PageMutationResponse, error)
	DeletePage(id int) error
}

var _ PageServicePort = (*PageService)(nil)

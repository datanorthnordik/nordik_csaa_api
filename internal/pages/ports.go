package pages

type PageServicePort interface {
	ListPages(filter PageListFilters) (*PageListResponse, error)
	GetPage(id int) (*PageDetailResponse, error)
	GetPageBySlug(slug string) (*PageDetailResponse, error)
	GetPageHeroImageContent(id int) (*PageHeroImageContent, error)
	GetPageCTABannerImageContent(sectionID int) (*PageSectionImageContent, error)
	GetPageDocumentContent(id int) (*PageDocumentContent, error)
	CreatePage(req SavePageRequest) (*PageMutationResponse, error)
	UpdatePage(id int, req SavePageRequest) (*PageMutationResponse, error)
	DeletePage(id int) error
}

var _ PageServicePort = (*PageService)(nil)

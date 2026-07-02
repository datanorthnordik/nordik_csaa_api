package bookshelf

type BookshelfServicePort interface {
	ListBooks(filter ListBookshelfFilter) (*BookshelfListResponse, error)
	GetBook(id int) (*BookshelfDetailResponse, error)
	GetBookContent(id int) (*BookshelfContent, error)
	GetAuthorImageContent(id int) (*BookshelfContent, error)
	GetCoverImageContent(id int) (*BookshelfContent, error)
	CreateBook(req SaveBookshelfEntryRequest, userID *int) (*BookshelfMutationResponse, error)
	UpdateBook(id int, req SaveBookshelfEntryRequest, userID *int) (*BookshelfMutationResponse, error)
	DeleteBook(id int) error
}

type ListBookshelfFilter struct {
	Page       int
	PageSize   int
	SearchTerm string
}

var _ BookshelfServicePort = (*BookshelfService)(nil)

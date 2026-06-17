package books

type BookServicePort interface {
	ListBooks() ([]BookSummaryResponse, error)
	GetBook(bookID int) (*BookDetailResponse, error)
	CreateBook(req SaveBookRequest) (*BookMutationResponse, error)
	UpdateBook(bookID int, req SaveBookRequest) (*BookMutationResponse, error)
	CreateBookVersion(bookID int, req SaveBookVersionRequest) (*BookVersionMutationResponse, error)
	UpdateBookVersion(bookID int, versionID int, req SaveBookVersionRequest) (*BookVersionMutationResponse, error)
	SetActiveVersion(bookID int, versionID int, userID *int) (*BookVersionMutationResponse, error)
	GetBookVersionDetail(bookID int, versionID int) (*BookVersionDetailResponse, error)
	UploadGeneratedPDF(bookID int, versionID int, input BookUploadInput, userID *int) (*BookVersionMutationResponse, error)
	GetSourcePDFContent(bookID int, versionID int) (*BookPDFContent, error)
	GetGeneratedPDFContent(bookID int, versionID int) (*BookPDFContent, error)
	GetSubmissionImageContent(bookID int, submissionID int) (*SubmissionImageContent, error)
	ListBookSubmissions(bookID int, filter ListBookSubmissionsFilter) ([]BookSubmissionResponse, error)
	GetBookSubmission(bookID int, submissionID int) (*BookSubmissionResponse, error)
	CreatePublicSubmission(bookID int, req SaveBookSubmissionRequest) (*BookSubmissionMutationResponse, error)
	UpdateBookSubmission(bookID int, submissionID int, req UpdateBookSubmissionRequest) (*BookSubmissionMutationResponse, error)
	ApproveBookSubmission(bookID int, submissionID int, userID *int) (*BookSubmissionMutationResponse, error)
	RejectBookSubmission(bookID int, submissionID int, req ReviewBookSubmissionRequest) (*BookSubmissionMutationResponse, error)
	ListPublicBooks() ([]PublicBookSummaryResponse, error)
	GetPublicBook(bookID int) (*PublicBookDetailResponse, error)
	GetPublicActivePDFContent(bookID int) (*BookPDFContent, error)
}

var _ BookServicePort = (*BookService)(nil)

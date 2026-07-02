package bookshelf

import "time"

type BookshelfEntry struct {
	ID                      int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Author                  string    `gorm:"size:255;not null" json:"author"`
	Title                   string    `gorm:"size:255;not null" json:"title"`
	BookLink                string    `gorm:"type:text;not null;default:'';column:book_link" json:"book_link"`
	AuthorBio               string    `gorm:"type:text;not null;default:'';column:author_bio" json:"author_bio"`
	BookTeaser              string    `gorm:"type:text;not null;default:'';column:book_teaser" json:"book_teaser"`
	Description             string    `gorm:"type:text;not null;default:''" json:"description"`
	BookFileName            string    `gorm:"size:255;column:book_file_name" json:"book_file_name"`
	BookGCPObjectKey        string    `gorm:"column:book_gcp_object_key" json:"book_gcp_object_key,omitempty"`
	BookFileURL             string    `gorm:"column:book_file_url" json:"book_file_url"`
	BookMimeType            string    `gorm:"size:255;column:book_mime_type" json:"book_mime_type"`
	BookFileSize            int64     `gorm:"column:book_file_size,omitempty" json:"book_file_size"`
	AuthorImageFileName     string    `gorm:"size:255;column:author_image_file_name" json:"author_image_file_name"`
	AuthorImageGCPObjectKey string    `gorm:"column:author_image_gcp_object_key" json:"author_image_gcp_object_key,omitempty"`
	AuthorImageFileURL      string    `gorm:"column:author_image_file_url" json:"author_image_file_url"`
	AuthorImageMimeType     string    `gorm:"size:255;column:author_image_mime_type" json:"author_image_mime_type"`
	AuthorImageFileSize     int64     `gorm:"column:author_image_file_size,omitempty" json:"author_image_file_size"`
	CoverImageFileName      string    `gorm:"size:255;column:cover_image_file_name" json:"cover_image_file_name"`
	CoverImageGCPObjectKey  string    `gorm:"column:cover_image_gcp_object_key" json:"cover_image_gcp_object_key,omitempty"`
	CoverImageFileURL       string    `gorm:"column:cover_image_file_url" json:"cover_image_file_url"`
	CoverImageMimeType      string    `gorm:"size:255;column:cover_image_mime_type" json:"cover_image_mime_type"`
	CoverImageFileSize      int64     `gorm:"column:cover_image_file_size,omitempty" json:"cover_image_file_size"`
	CreatedBy               *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy               *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type BookshelfUploadInput struct {
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	Content      []byte `json:"-"`
	FileURL      string `json:"file_url"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SaveBookshelfEntryRequest struct {
	Author            string                `json:"author" binding:"required"`
	Title             string                `json:"title" binding:"required"`
	BookLink          string                `json:"book_link"`
	AuthorBio         string                `json:"author_bio"`
	BookTeaser        string                `json:"book_teaser"`
	Description       string                `json:"description" binding:"required"`
	BookUpload        *BookshelfUploadInput `json:"book_upload,omitempty"`
	AuthorImage       *BookshelfUploadInput `json:"author_image,omitempty"`
	CoverImage        *BookshelfUploadInput `json:"cover_image,omitempty"`
	RemoveAuthorImage bool                  `json:"remove_author_image"`
	RemoveCoverImage  bool                  `json:"remove_cover_image"`
}

type BookshelfMutationResponse struct {
	ID        int       `json:"id"`
	Author    string    `json:"author"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BookshelfListItem struct {
	ID                    int       `json:"id"`
	Author                string    `json:"author"`
	Title                 string    `json:"title"`
	BookLink              string    `json:"book_link"`
	AuthorBio             string    `json:"author_bio"`
	BookTeaser            string    `json:"book_teaser"`
	Description           string    `json:"description"`
	BookFileName          string    `json:"book_file_name"`
	BookMimeType          string    `json:"book_mime_type"`
	BookFileSize          int64     `json:"book_file_size"`
	BookContentURL        string    `json:"book_content_url"`
	AuthorImageFileName   string    `json:"author_image_file_name"`
	AuthorImageMimeType   string    `json:"author_image_mime_type"`
	AuthorImageFileSize   int64     `json:"author_image_file_size"`
	HasAuthorImage        bool      `json:"has_author_image"`
	AuthorImageContentURL string    `json:"author_image_content_url"`
	CoverImageFileName    string    `json:"cover_image_file_name"`
	CoverImageMimeType    string    `json:"cover_image_mime_type"`
	CoverImageFileSize    int64     `json:"cover_image_file_size"`
	HasCoverImage         bool      `json:"has_cover_image"`
	CoverImageContentURL  string    `json:"cover_image_content_url"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type BookshelfDetailResponse = BookshelfListItem

type BookshelfListPageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type BookshelfListAppliedFilters struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	SearchTerm string `json:"search_term"`
}

type BookshelfListSummary struct {
	WithCoverCount    int64 `json:"with_cover_count"`
	WithoutCoverCount int64 `json:"without_cover_count"`
}

type BookshelfListResponse struct {
	Items      []BookshelfListItem         `json:"items"`
	Pagination BookshelfListPageMeta       `json:"pagination"`
	Summary    BookshelfListSummary        `json:"summary"`
	Applied    BookshelfListAppliedFilters `json:"applied_filters"`
}

type BookshelfContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

func (BookshelfEntry) TableName() string {
	return "bookshelf_entries"
}

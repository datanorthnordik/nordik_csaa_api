package books

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

const (
	BookFieldInputTypeSingleLine = "single_line"
	BookFieldInputTypeRichText   = "rich_text"

	BookFieldPlacementHeading = "heading"
	BookFieldPlacementBody    = "body"

	BookSubmissionStatusPending  = "pending"
	BookSubmissionStatusApproved = "approved"
	BookSubmissionStatusRejected = "rejected"
)

var defaultBookLayoutSettings = json.RawMessage(`{
  "content_mask": {
    "x": 54,
    "y": 92,
    "width": 392,
    "height": 484,
    "background_color": "#ffffff"
  },
  "heading_area": {
    "x": 78,
    "y": 114,
    "width": 316,
    "height": 86,
    "font_size": 19,
    "line_height": 1.2,
    "text_align": "left"
  },
  "body_area": {
    "x": 78,
    "y": 214,
    "width": 280,
    "height": 314,
    "font_size": 11,
    "line_height": 1.35,
    "text_align": "left"
  },
  "image_area": {
    "x": 316,
    "y": 422,
    "width": 108,
    "height": 108
  },
  "section_mask": {
    "x": 70,
    "y": 228,
    "width": 360,
    "height": 114,
    "background_color": "#ffffff"
  },
  "section_title_area": {
    "x": 90,
    "y": 248,
    "width": 320,
    "height": 74,
    "font_size": 28,
    "line_height": 1.1,
    "text_align": "center"
  }
}`)

type Book struct {
	ID                      int            `gorm:"primaryKey;autoIncrement" json:"id"`
	Title                   string         `gorm:"size:255;not null;column:title" json:"title"`
	Description             string         `gorm:"type:text;column:description" json:"description"`
	AdminNotificationEmails pq.StringArray `gorm:"type:text[];not null;default:'{}';column:admin_notification_emails" json:"admin_notification_emails"`
	ActiveVersionID         *int           `gorm:"column:active_version_id" json:"active_version_id,omitempty"`
	CreatedBy               *int           `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy               *int           `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
}

type BookVersion struct {
	ID                        int             `gorm:"primaryKey;autoIncrement" json:"id"`
	BookID                    int             `gorm:"not null;column:book_id" json:"book_id"`
	VersionNumber             int             `gorm:"not null;column:version_number" json:"version_number"`
	SourcePageCount           int             `gorm:"not null;column:source_page_count" json:"source_page_count"`
	ContentTemplatePageNumber int             `gorm:"not null;column:content_template_page_number" json:"content_template_page_number"`
	SectionTemplatePageNumber int             `gorm:"not null;column:section_template_page_number" json:"section_template_page_number"`
	AllowPageImage            bool            `gorm:"not null;default:false;column:allow_page_image" json:"allow_page_image"`
	AllowNewSections          bool            `gorm:"not null;default:true;column:allow_new_sections" json:"allow_new_sections"`
	LayoutSettings            json.RawMessage `gorm:"type:jsonb;not null;column:layout_settings" json:"layout_settings"`
	SourcePDFFileName         string          `gorm:"size:255;not null;column:source_pdf_file_name" json:"source_pdf_file_name"`
	SourcePDFFileURL          string          `gorm:"column:source_pdf_file_url" json:"source_pdf_file_url"`
	SourcePDFStorageURI       string          `gorm:"column:source_pdf_storage_uri" json:"source_pdf_storage_uri"`
	SourcePDFObjectKey        string          `gorm:"column:source_pdf_object_key" json:"source_pdf_object_key"`
	GeneratedPDFFileName      string          `gorm:"size:255;not null;column:generated_pdf_file_name" json:"generated_pdf_file_name"`
	GeneratedPDFFileURL       string          `gorm:"column:generated_pdf_file_url" json:"generated_pdf_file_url"`
	GeneratedPDFStorageURI    string          `gorm:"column:generated_pdf_storage_uri" json:"generated_pdf_storage_uri"`
	GeneratedPDFObjectKey     string          `gorm:"column:generated_pdf_object_key" json:"generated_pdf_object_key"`
	LastGeneratedAt           *time.Time      `gorm:"column:last_generated_at" json:"last_generated_at,omitempty"`
	CreatedBy                 *int            `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy                 *int            `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
}

type BookVersionSection struct {
	ID               int        `gorm:"primaryKey;autoIncrement" json:"id"`
	BookVersionID    int        `gorm:"not null;column:book_version_id" json:"book_version_id"`
	Name             string     `gorm:"size:150;not null;column:name" json:"name"`
	SourceStartPage  *int       `gorm:"column:source_start_page" json:"source_start_page,omitempty"`
	SourceEndPage    *int       `gorm:"column:source_end_page" json:"source_end_page,omitempty"`
	CurrentStartPage int        `gorm:"not null;default:0;column:current_start_page" json:"current_start_page"`
	CurrentEndPage   int        `gorm:"not null;default:0;column:current_end_page" json:"current_end_page"`
	SortOrder        int        `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type BookVersionField struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	BookVersionID int     `gorm:"not null;column:book_version_id" json:"book_version_id"`
	FieldKey    string    `gorm:"size:150;not null;column:field_key" json:"field_key"`
	Label       string    `gorm:"size:150;not null;column:label" json:"label"`
	InputType   string    `gorm:"size:30;not null;column:input_type" json:"input_type"`
	Placement   string    `gorm:"size:30;not null;column:placement" json:"placement"`
	ShowLabel   bool      `gorm:"not null;default:true;column:show_label" json:"show_label"`
	IsRequired  bool      `gorm:"not null;default:false;column:is_required" json:"is_required"`
	IsEmailField bool     `gorm:"not null;default:false;column:is_email_field" json:"is_email_field"`
	SortOrder   int       `gorm:"not null;default:0;column:sort_order" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BookSubmission struct {
	ID                int        `gorm:"primaryKey;autoIncrement" json:"id"`
	BookID            int        `gorm:"not null;column:book_id" json:"book_id"`
	BookVersionID     int        `gorm:"not null;column:book_version_id" json:"book_version_id"`
	TargetSectionID   *int       `gorm:"column:target_section_id" json:"target_section_id,omitempty"`
	NewSectionName    string     `gorm:"size:150;column:new_section_name" json:"new_section_name,omitempty"`
	Status            string     `gorm:"size:20;not null;default:pending;column:status" json:"status"`
	SubmitterEmail    string     `gorm:"size:255;column:submitter_email" json:"submitter_email,omitempty"`
	ImageFileName     string     `gorm:"size:255;column:image_file_name" json:"image_file_name,omitempty"`
	ImageFileURL      string     `gorm:"column:image_file_url" json:"image_file_url,omitempty"`
	ImageStorageURI   string     `gorm:"column:image_storage_uri" json:"image_storage_uri,omitempty"`
	ImageObjectKey    string     `gorm:"column:image_object_key" json:"image_object_key,omitempty"`
	ImageMimeType     string     `gorm:"size:255;column:image_mime_type" json:"image_mime_type,omitempty"`
	ImageFileSize     int64      `gorm:"column:image_file_size" json:"image_file_size,omitempty"`
	ReviewedBy        *int       `gorm:"column:reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time `gorm:"column:reviewed_at" json:"reviewed_at,omitempty"`
	RejectionReason   string     `gorm:"type:text;column:rejection_reason" json:"rejection_reason,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type BookSubmissionValue struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	BookSubmissionID int       `gorm:"not null;column:book_submission_id" json:"book_submission_id"`
	BookFieldID      int       `gorm:"not null;column:book_field_id" json:"book_field_id"`
	Value            string    `gorm:"type:text;column:value" json:"value"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BookUploadInput struct {
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	Content      []byte `json:"-"`
	FileURL      string `json:"file_url"`
	StorageURI   string `json:"storage_uri"`
	ObjectKey    string `json:"object_key"`
	GCPObjectKey string `json:"gcp_object_key"`
}

type SaveBookRequest struct {
	Title                   string   `json:"title" binding:"required"`
	Description             string   `json:"description"`
	AdminNotificationEmails []string `json:"admin_notification_emails"`
	CreatedBy               *int     `json:"created_by,omitempty"`
	UpdatedBy               *int     `json:"updated_by,omitempty"`
}

type SaveBookVersionSectionRequest struct {
	ID              int  `json:"id,omitempty"`
	Name            string `json:"name"`
	SourceStartPage *int `json:"source_start_page,omitempty"`
	SourceEndPage   *int `json:"source_end_page,omitempty"`
}

type SaveBookVersionFieldRequest struct {
	ID           int    `json:"id,omitempty"`
	Label        string `json:"label"`
	InputType    string `json:"input_type"`
	Placement    string `json:"placement"`
	ShowLabel    bool   `json:"show_label"`
	IsRequired   bool   `json:"is_required"`
	IsEmailField bool   `json:"is_email_field"`
}

type SaveBookVersionRequest struct {
	SourcePageCount           int                             `json:"source_page_count"`
	ContentTemplatePageNumber int                             `json:"content_template_page_number"`
	SectionTemplatePageNumber int                             `json:"section_template_page_number"`
	AllowPageImage            bool                            `json:"allow_page_image"`
	AllowNewSections          bool                            `json:"allow_new_sections"`
	LayoutSettings            json.RawMessage                 `json:"layout_settings"`
	Sections                  []SaveBookVersionSectionRequest `json:"sections"`
	Fields                    []SaveBookVersionFieldRequest   `json:"fields"`
	ActivateImmediately       bool                            `json:"activate_immediately"`
	SourcePDF                 *BookUploadInput                `json:"source_pdf,omitempty"`
	GeneratedPDF              *BookUploadInput                `json:"generated_pdf,omitempty"`
	CreatedBy                 *int                            `json:"created_by,omitempty"`
	UpdatedBy                 *int                            `json:"updated_by,omitempty"`
}

type BookSubmissionValueInput struct {
	FieldID int    `json:"field_id"`
	Value   string `json:"value"`
}

type SaveBookSubmissionRequest struct {
	TargetSectionID *int                      `json:"target_section_id,omitempty"`
	NewSectionName  string                    `json:"new_section_name"`
	FieldValues     []BookSubmissionValueInput `json:"field_values"`
	Image           *BookUploadInput          `json:"image,omitempty"`
}

type UpdateBookSubmissionRequest struct {
	TargetSectionID *int                      `json:"target_section_id,omitempty"`
	NewSectionName  string                    `json:"new_section_name"`
	FieldValues     []BookSubmissionValueInput `json:"field_values"`
	Image           *BookUploadInput          `json:"image,omitempty"`
	RemoveImage     bool                      `json:"remove_image"`
}

type ReviewBookSubmissionRequest struct {
	RejectionReason string `json:"rejection_reason"`
	ReviewedBy      *int   `json:"reviewed_by,omitempty"`
}

type BookSummaryResponse struct {
	ID                   int                 `json:"id"`
	Title                string              `json:"title"`
	Description          string              `json:"description"`
	ActiveVersionID      *int                `json:"active_version_id,omitempty"`
	ActiveVersionNumber  *int                `json:"active_version_number,omitempty"`
	VersionCount         int                 `json:"version_count"`
	PendingSubmissionCount int               `json:"pending_submission_count"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

type BookMutationResponse struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BookVersionSummaryResponse struct {
	ID                      int        `json:"id"`
	VersionNumber           int        `json:"version_number"`
	IsActive                bool       `json:"is_active"`
	SourcePageCount         int        `json:"source_page_count"`
	SectionsCount           int        `json:"sections_count"`
	FieldsCount             int        `json:"fields_count"`
	ApprovedSubmissionCount int        `json:"approved_submission_count"`
	PendingSubmissionCount  int        `json:"pending_submission_count"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	LastGeneratedAt         *time.Time `json:"last_generated_at,omitempty"`
}

type BookDetailResponse struct {
	ID                      int                         `json:"id"`
	Title                   string                      `json:"title"`
	Description             string                      `json:"description"`
	AdminNotificationEmails []string                    `json:"admin_notification_emails"`
	ActiveVersionID         *int                        `json:"active_version_id,omitempty"`
	Versions                []BookVersionSummaryResponse `json:"versions"`
	CreatedAt               time.Time                   `json:"created_at"`
	UpdatedAt               time.Time                   `json:"updated_at"`
}

type BookVersionSectionResponse struct {
	ID               int       `json:"id"`
	Name             string    `json:"name"`
	SourceStartPage  *int      `json:"source_start_page,omitempty"`
	SourceEndPage    *int      `json:"source_end_page,omitempty"`
	CurrentStartPage int       `json:"current_start_page"`
	CurrentEndPage   int       `json:"current_end_page"`
	SortOrder        int       `json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BookVersionFieldResponse struct {
	ID           int       `json:"id"`
	Label        string    `json:"label"`
	InputType    string    `json:"input_type"`
	Placement    string    `json:"placement"`
	ShowLabel    bool      `json:"show_label"`
	IsRequired   bool      `json:"is_required"`
	IsEmailField bool      `json:"is_email_field"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BookSubmissionImageResponse struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
	FetchURL string `json:"fetch_url"`
}

type BookSubmissionValueResponse struct {
	FieldID      int    `json:"field_id"`
	Label        string `json:"label"`
	InputType    string `json:"input_type"`
	Placement    string `json:"placement"`
	ShowLabel    bool   `json:"show_label"`
	IsEmailField bool   `json:"is_email_field"`
	Value        string `json:"value"`
}

type BookSubmissionResponse struct {
	ID               int                         `json:"id"`
	BookID           int                         `json:"book_id"`
	BookVersionID    int                         `json:"book_version_id"`
	TargetSectionID  *int                        `json:"target_section_id,omitempty"`
	TargetSectionName string                     `json:"target_section_name"`
	NewSectionName   string                      `json:"new_section_name"`
	Status           string                      `json:"status"`
	SubmitterEmail   string                      `json:"submitter_email"`
	Image            *BookSubmissionImageResponse `json:"image,omitempty"`
	FieldValues      []BookSubmissionValueResponse `json:"field_values"`
	ReviewedBy       *int                        `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time                  `json:"reviewed_at,omitempty"`
	RejectionReason  string                      `json:"rejection_reason"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

type BookVersionDetailResponse struct {
	ID                        int                          `json:"id"`
	BookID                    int                          `json:"book_id"`
	VersionNumber             int                          `json:"version_number"`
	IsActive                  bool                         `json:"is_active"`
	SourcePageCount           int                          `json:"source_page_count"`
	ContentTemplatePageNumber int                          `json:"content_template_page_number"`
	SectionTemplatePageNumber int                          `json:"section_template_page_number"`
	AllowPageImage            bool                         `json:"allow_page_image"`
	AllowNewSections          bool                         `json:"allow_new_sections"`
	LayoutSettings            json.RawMessage              `json:"layout_settings"`
	SourcePDFFetchURL         string                       `json:"source_pdf_fetch_url"`
	GeneratedPDFFetchURL      string                       `json:"generated_pdf_fetch_url"`
	Sections                  []BookVersionSectionResponse `json:"sections"`
	Fields                    []BookVersionFieldResponse   `json:"fields"`
	ApprovedSubmissions       []BookSubmissionResponse     `json:"approved_submissions"`
	LastGeneratedAt           *time.Time                   `json:"last_generated_at,omitempty"`
	CreatedAt                 time.Time                    `json:"created_at"`
	UpdatedAt                 time.Time                    `json:"updated_at"`
}

type BookVersionMutationResponse struct {
	ID            int       `json:"id"`
	BookID        int       `json:"book_id"`
	VersionNumber int       `json:"version_number"`
	IsActive      bool      `json:"is_active"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type BookSubmissionMutationResponse struct {
	ID        int       `json:"id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BookPDFContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

type SubmissionImageContent struct {
	Content     []byte
	ContentType string
	FileName    string
}

type ListBookSubmissionsFilter struct {
	VersionID int
	Status    string
}

type PublicBookSummaryResponse struct {
	ID              int   `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	ActiveVersionID *int   `json:"active_version_id,omitempty"`
}

type PublicBookDetailResponse struct {
	ID          int                         `json:"id"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Version     PublicBookActiveVersionResponse `json:"version"`
}

type PublicBookActiveVersionResponse struct {
	ID               int                          `json:"id"`
	VersionNumber    int                          `json:"version_number"`
	PDFContentURL    string                       `json:"pdf_content_url"`
	AllowPageImage   bool                         `json:"allow_page_image"`
	AllowNewSections bool                         `json:"allow_new_sections"`
	Sections         []BookVersionSectionResponse `json:"sections"`
	Fields           []BookVersionFieldResponse   `json:"fields"`
}

func (Book) TableName() string {
	return "books"
}

func (BookVersion) TableName() string {
	return "book_versions"
}

func (BookVersionSection) TableName() string {
	return "book_version_sections"
}

func (BookVersionField) TableName() string {
	return "book_version_fields"
}

func (BookSubmission) TableName() string {
	return "book_submissions"
}

func (BookSubmissionValue) TableName() string {
	return "book_submission_values"
}

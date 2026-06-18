package knowledgecenter

import "time"

const (
	KnowledgeCenterSubmissionTypePost  = "post"
	KnowledgeCenterSubmissionTypeVideo = "video"
	KnowledgeCenterSubmissionTypeBoth  = "both"

	KnowledgeCenterSubmissionStatusOpen      = "open"
	KnowledgeCenterSubmissionStatusCompleted = "completed"
)

type KnowledgeCenterSubmission struct {
	ID                int        `gorm:"primaryKey;autoIncrement" json:"id"`
	SubmitterName     string     `gorm:"size:255;not null;column:submitter_name" json:"submitter_name"`
	SubmitterEmail    string     `gorm:"size:255;not null;column:submitter_email" json:"submitter_email"`
	SubmitterPhone    string     `gorm:"size:80;column:submitter_phone" json:"submitter_phone"`
	SubmissionType    string     `gorm:"size:20;not null;column:submission_type" json:"submission_type"`
	Message           string     `gorm:"type:text;not null;column:message" json:"message"`
	Status            string     `gorm:"size:20;not null;default:open;column:status" json:"status"`
	CompletionNotes   string     `gorm:"type:text;column:completion_notes" json:"completion_notes"`
	CompletedByUserID *int       `gorm:"column:completed_by_user_id" json:"completed_by_user_id,omitempty"`
	CompletedByName   string     `gorm:"size:255;column:completed_by_name" json:"completed_by_name"`
	CompletedByEmail  string     `gorm:"size:255;column:completed_by_email" json:"completed_by_email"`
	CompletedAt       *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type CreateKnowledgeCenterSubmissionRequest struct {
	SubmitterName  string `json:"name"`
	SubmitterEmail string `json:"email"`
	SubmitterPhone string `json:"phone"`
	SubmissionType string `json:"type"`
	Message        string `json:"message"`
}

type CompleteKnowledgeCenterSubmissionRequest struct {
	CompletionNotes string `json:"completion_notes"`
}

type KnowledgeCenterCompletedByResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type KnowledgeCenterSubmissionResponse struct {
	ID              int                                 `json:"id"`
	SubmitterName   string                              `json:"submitter_name"`
	SubmitterEmail  string                              `json:"submitter_email"`
	SubmitterPhone  string                              `json:"submitter_phone"`
	SubmissionType  string                              `json:"submission_type"`
	Message         string                              `json:"message"`
	Status          string                              `json:"status"`
	CompletionNotes string                              `json:"completion_notes"`
	CompletedBy     *KnowledgeCenterCompletedByResponse `json:"completed_by,omitempty"`
	CompletedAt     *time.Time                          `json:"completed_at,omitempty"`
	CreatedAt       time.Time                           `json:"created_at"`
	UpdatedAt       time.Time                           `json:"updated_at"`
}

type KnowledgeCenterSubmissionsPageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

type KnowledgeCenterSubmissionsSummary struct {
	OpenCount      int64 `json:"open_count"`
	CompletedCount int64 `json:"completed_count"`
}

type KnowledgeCenterSubmissionsAppliedFilters struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	SearchTerm string `json:"search_term"`
	Status     string `json:"status"`
}

type KnowledgeCenterSubmissionsListResponse struct {
	Items      []KnowledgeCenterSubmissionResponse      `json:"items"`
	Pagination KnowledgeCenterSubmissionsPageMeta       `json:"pagination"`
	Summary    KnowledgeCenterSubmissionsSummary        `json:"summary"`
	Applied    KnowledgeCenterSubmissionsAppliedFilters `json:"applied_filters"`
}

type ListKnowledgeCenterSubmissionsFilter struct {
	Page       int
	PageSize   int
	SearchTerm string
	Status     string
}

func (KnowledgeCenterSubmission) TableName() string {
	return "knowledge_center_submissions"
}

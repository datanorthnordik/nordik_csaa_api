package knowledgecenter

type KnowledgeCenterServicePort interface {
	ListSubmissions(filter ListKnowledgeCenterSubmissionsFilter) (*KnowledgeCenterSubmissionsListResponse, error)
	GetSubmission(id int) (*KnowledgeCenterSubmissionResponse, error)
	CreatePublicSubmission(req CreateKnowledgeCenterSubmissionRequest) (*KnowledgeCenterSubmissionResponse, error)
	MarkSubmissionCompleted(id int, req CompleteKnowledgeCenterSubmissionRequest, userID *int) (*KnowledgeCenterSubmissionResponse, error)
}

var _ KnowledgeCenterServicePort = (*KnowledgeCenterService)(nil)

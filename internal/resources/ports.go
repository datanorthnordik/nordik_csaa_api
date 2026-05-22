package resources

type ResourceServicePort interface {
	ListResources(filter ListResourcesFilter) (*ResourceListResponse, error)
	GetResource(id int) (*ResourceDetailResponse, error)
	GetResourceContent(id int) (*ResourceContent, error)
	CreateResource(req SaveResourceRequest, userID *int) (*ResourceMutationResponse, error)
	UpdateResource(id int, req SaveResourceRequest, userID *int) (*ResourceMutationResponse, error)
	DeleteResource(id int) error
}

type ListResourcesFilter struct {
	Page       int
	PageSize   int
	SearchTerm string
	Category   string
	FileType   string
}

var _ ResourceServicePort = (*ResourceService)(nil)

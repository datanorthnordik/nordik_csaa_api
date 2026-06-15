package video

type VideoServicePort interface {
	ListVideoPackages() (*VideoPackageListResponse, error)
	GetVideoPackage(id int) (*VideoPackageDetailResponse, error)
	GetVideoTeaserContent(id int, itemID int) (*VideoMediaContent, error)
	CreateVideoPackage(req SaveVideoPackageRequest, userID *int) (*VideoPackageMutationResponse, error)
	UpdateVideoPackage(id int, req UpdateVideoPackageRequest, userID *int) (*VideoPackageMutationResponse, error)
	DeleteVideoPackage(id int) error
	AddVideoItems(id int, req AddVideoItemsRequest, userID *int) (*AddVideoItemsResponse, error)
	UpdateVideoItem(id int, itemID int, req UpdateVideoItemRequest, userID *int) (*VideoItemResponse, error)
	DeleteVideoItem(id int, itemID int) (*DeleteVideoItemResponse, error)
}

var _ VideoServicePort = (*VideoService)(nil)

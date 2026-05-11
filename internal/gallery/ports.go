package gallery

type GalleryServicePort interface {
	ListGalleries() (*GalleryListResponse, error)
	GetGallery(id int) (*GalleryDetailResponse, error)
	GetGalleryCoverContent(id int) (*GalleryMediaContent, error)
	GetGalleryImageContent(id int, imageID int) (*GalleryMediaContent, error)
	CreateGallery(req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error)
	UpdateGallery(id int, req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error)
	DeleteGallery(id int) error
	AddGalleryImages(id int, req AddGalleryImagesRequest, userID *int) (*AddGalleryImagesResponse, error)
	UpdateGalleryImage(id int, imageID int, req UpdateGalleryImageRequest) (*GalleryAssetResponse, error)
	ReorderGalleryImages(id int, imageIDs []int) (*ReorderGalleryImagesResponse, error)
	DeleteGalleryImages(id int, storageURLs []string) (*DeleteGalleryImagesResponse, error)
}

var _ GalleryServicePort = (*GalleryService)(nil)

package gallery

type GalleryServicePort interface {
	CreateGallery(req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error)
	UpdateGallery(id int, req SaveGalleryRequest, userID *int) (*GalleryMutationResponse, error)
	DeleteGallery(id int) error
	AddGalleryImages(id int, req AddGalleryImagesRequest, userID *int) (*DeleteGalleryImagesResponse, error)
	DeleteGalleryImages(id int, storageURLs []string) (*DeleteGalleryImagesResponse, error)
}

var _ GalleryServicePort = (*GalleryService)(nil)

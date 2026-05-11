package gallery

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, gs GalleryServicePort, protected ...gin.HandlerFunc) {
	controller := &GalleryController{GalleryService: gs}

	group := r.Group("/api/galleries")
	{
		getHandlers := withProtected(controller.ListGalleries, protected...)
		getByIDHandlers := withProtected(controller.GetGallery, protected...)
		getCoverHandlers := withProtected(controller.GetGalleryCoverContent, protected...)
		getImageContentHandlers := withProtected(controller.GetGalleryImageContent, protected...)
		postHandlers := withProtected(controller.CreateGallery, protected...)
		putHandlers := withProtected(controller.UpdateGallery, protected...)
		deleteHandlers := withProtected(controller.DeleteGallery, protected...)
		addImagesHandlers := withProtected(controller.AddGalleryImages, protected...)
		updateImageHandlers := withProtected(controller.UpdateGalleryImage, protected...)
		reorderImagesHandlers := withProtected(controller.ReorderGalleryImages, protected...)
		deleteImagesHandlers := withProtected(controller.DeleteGalleryImages, protected...)

		group.GET("", getHandlers...)
		group.GET("/:id", getByIDHandlers...)
		group.GET("/:id/cover/content", getCoverHandlers...)
		group.GET("/:id/images/:imageId/content", getImageContentHandlers...)
		group.POST("", postHandlers...)
		group.PUT("/:id", putHandlers...)
		group.DELETE("/:id", deleteHandlers...)
		group.POST("/:id/images", addImagesHandlers...)
		group.PATCH("/:id/images/:imageId", updateImageHandlers...)
		group.PUT("/:id/images/order", reorderImagesHandlers...)
		group.DELETE("/:id/images", deleteImagesHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

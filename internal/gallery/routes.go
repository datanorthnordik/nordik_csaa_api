package gallery

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, gs GalleryServicePort, protected ...gin.HandlerFunc) {
	controller := &GalleryController{GalleryService: gs}

	group := r.Group("/api/galleries")
	{
		postHandlers := withProtected(controller.CreateGallery, protected...)
		putHandlers := withProtected(controller.UpdateGallery, protected...)
		deleteHandlers := withProtected(controller.DeleteGallery, protected...)
		addImagesHandlers := withProtected(controller.AddGalleryImages, protected...)
		deleteImagesHandlers := withProtected(controller.DeleteGalleryImages, protected...)

		group.POST("", postHandlers...)
		group.PUT("/:id", putHandlers...)
		group.DELETE("/:id", deleteHandlers...)
		group.POST("/:id/images", addImagesHandlers...)
		group.DELETE("/:id/images", deleteImagesHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

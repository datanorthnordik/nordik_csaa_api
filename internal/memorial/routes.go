package memorial

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, ms MemorialServicePort, protected ...gin.HandlerFunc) {
	controller := &MemorialController{MemorialService: ms}

	group := r.Group("/api/memorial")
	{
		listHandlers := withProtected(controller.ListMemorials, protected...)
		getHandlers := withProtected(controller.GetMemorial, protected...)
		portraitHandlers := withProtected(controller.GetMemorialPortraitContent, protected...)
		galleryHandlers := withProtected(controller.GetMemorialGalleryImageContent, protected...)
		postHandlers := withProtected(controller.CreateMemorial, protected...)
		putHandlers := withProtected(controller.UpdateMemorial, protected...)
		deleteHandlers := withProtected(controller.DeleteMemorial, protected...)

		group.GET("", listHandlers...)
		group.GET("/:id", getHandlers...)
		group.GET("/:id/portrait/content", portraitHandlers...)
		group.GET("/:id/gallery/:mediaId/content", galleryHandlers...)
		group.POST("", postHandlers...)
		group.PUT("/:id", putHandlers...)
		group.DELETE("/:id", deleteHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

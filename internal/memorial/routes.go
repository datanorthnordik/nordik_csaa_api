package memorial

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, ms MemorialServicePort, protected ...gin.HandlerFunc) {
	controller := &MemorialController{MemorialService: ms}

	group := r.Group("/api/memorial")
	{
		postHandlers := withProtected(controller.CreateMemorial, protected...)
		putHandlers := withProtected(controller.UpdateMemorial, protected...)
		deleteHandlers := withProtected(controller.DeleteMemorial, protected...)

		group.GET("", controller.ListMemorials)
		group.GET("/:id", controller.GetMemorial)
		group.GET("/:id/portrait/content", controller.GetMemorialPortraitContent)
		group.GET("/:id/gallery/:mediaId/content", controller.GetMemorialGalleryImageContent)
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

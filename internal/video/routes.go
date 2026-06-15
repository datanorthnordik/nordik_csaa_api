package video

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, vs VideoServicePort, protected ...gin.HandlerFunc) {
	controller := &VideoController{VideoService: vs}

	group := r.Group("/api/videos")
	{
		postHandlers := withProtected(controller.CreateVideoPackage, protected...)
		putHandlers := withProtected(controller.UpdateVideoPackage, protected...)
		deleteHandlers := withProtected(controller.DeleteVideoPackage, protected...)
		addItemsHandlers := withProtected(controller.AddVideoItems, protected...)
		updateItemHandlers := withProtected(controller.UpdateVideoItem, protected...)
		deleteItemHandlers := withProtected(controller.DeleteVideoItem, protected...)

		group.GET("", controller.ListVideoPackages)
		group.GET("/:id", controller.GetVideoPackage)
		group.GET("/:id/items/:itemId/teaser/content", controller.GetVideoTeaserContent)
		group.POST("", postHandlers...)
		group.PUT("/:id", putHandlers...)
		group.DELETE("/:id", deleteHandlers...)
		group.POST("/:id/items", addItemsHandlers...)
		group.PATCH("/:id/items/:itemId", updateItemHandlers...)
		group.DELETE("/:id/items/:itemId", deleteItemHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

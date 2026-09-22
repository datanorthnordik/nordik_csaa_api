package recordings

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, rs RecordingServicePort, protected ...gin.HandlerFunc) {
	controller := &RecordingController{RecordingService: rs}

	group := r.Group("/api/recordings")
	{
		postHandlers := withProtected(controller.CreateRecordingCollection, protected...)
		putHandlers := withProtected(controller.UpdateRecordingCollection, protected...)
		deleteHandlers := withProtected(controller.DeleteRecordingCollection, protected...)
		addItemsHandlers := withProtected(controller.AddRecordingItems, protected...)
		updateItemHandlers := withProtected(controller.UpdateRecordingItem, protected...)
		deleteItemHandlers := withProtected(controller.DeleteRecordingItem, protected...)

		group.GET("", controller.ListRecordingCollections)
		group.GET("/:id", controller.GetRecordingCollection)
		group.GET("/:id/items/:itemId/content", controller.GetRecordingItemContent)
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

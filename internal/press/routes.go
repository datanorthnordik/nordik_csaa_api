package press

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, ps PressServicePort, protected ...gin.HandlerFunc) {
	controller := &PressController{PressService: ps}

	publicGroup := r.Group("/api/press")
	{
		// Public GET endpoints - read-only, no authentication required
		publicGroup.GET("", controller.ListPressEntries)
		publicGroup.GET("/:id", controller.GetPressEntry)
		publicGroup.GET("/:id/media/:mediaId/content", controller.GetPressMediaContent)

		// Protected endpoints - require authentication
		postHandlers := withProtected(controller.CreatePressEntry, protected...)
		putHandlers := withProtected(controller.UpdatePressEntry, protected...)
		deleteHandlers := withProtected(controller.DeletePressEntry, protected...)
		addMediaHandlers := withProtected(controller.AddPressMedia, protected...)
		updateMediaHandlers := withProtected(controller.UpdatePressMedia, protected...)
		reorderMediaHandlers := withProtected(controller.ReorderPressMedia, protected...)
		deleteMediaHandlers := withProtected(controller.DeletePressMedia, protected...)

		publicGroup.POST("", postHandlers...)
		publicGroup.PUT("/:id", putHandlers...)
		publicGroup.DELETE("/:id", deleteHandlers...)
		publicGroup.POST("/:id/media", addMediaHandlers...)
		publicGroup.PATCH("/:id/media/:mediaId", updateMediaHandlers...)
		publicGroup.PUT("/:id/media/order", reorderMediaHandlers...)
		publicGroup.DELETE("/:id/media", deleteMediaHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

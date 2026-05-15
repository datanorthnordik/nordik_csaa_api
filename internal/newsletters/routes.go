package newsletters

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, ns NewsletterServicePort, protected ...gin.HandlerFunc) {
	controller := &NewsletterController{NewsletterService: ns}

	publicGroup := r.Group("/api/newsletters")
	{
		publicGroup.GET("", controller.ListNewsletterEntries)
		publicGroup.GET("/:id", controller.GetNewsletterEntry)
		publicGroup.GET("/:id/media/:mediaId/content", controller.GetNewsletterMediaContent)

		postHandlers := withProtected(controller.CreateNewsletterEntry, protected...)
		putHandlers := withProtected(controller.UpdateNewsletterEntry, protected...)
		deleteHandlers := withProtected(controller.DeleteNewsletterEntry, protected...)
		addMediaHandlers := withProtected(controller.AddNewsletterMedia, protected...)
		updateMediaHandlers := withProtected(controller.UpdateNewsletterMedia, protected...)
		reorderMediaHandlers := withProtected(controller.ReorderNewsletterMedia, protected...)
		deleteMediaHandlers := withProtected(controller.DeleteNewsletterMedia, protected...)

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

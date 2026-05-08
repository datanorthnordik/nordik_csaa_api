package events

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, es EventServicePort, protected ...gin.HandlerFunc) {
	controller := &EventController{EventService: es}

	publicGroup := r.Group("/api/events")
	{
		publicGroup.GET("/location", controller.ListSavedLocations)
		publicGroup.GET("/locations", controller.ListSavedLocations)
		publicGroup.GET("/galleries", controller.ListGalleries)
		publicGroup.GET("", controller.ListEvents)
		publicGroup.GET("/:id", controller.GetEvent)
		publicGroup.GET("/:id/media/:mediaId/content", controller.GetEventMediaContent)

		postHandlers := withProtected(controller.CreateEvent, protected...)
		putHandlers := withProtected(controller.UpdateEvent, protected...)
		deleteHandlers := withProtected(controller.DeleteEvent, protected...)
		deleteDocumentHandlers := withProtected(controller.DeleteEventDocument, protected...)
		deleteAllDocumentHandlers := withProtected(controller.DeleteAllEventDocuments, protected...)
		deletePhotoHandlers := withProtected(controller.DeleteEventPhoto, protected...)

		publicGroup.POST("", postHandlers...)
		publicGroup.PUT("/:id", putHandlers...)
		publicGroup.DELETE("/:id", deleteHandlers...)
		publicGroup.DELETE("/:id/documents/:mediaId", deleteDocumentHandlers...)
		publicGroup.DELETE("/:id/documents", deleteAllDocumentHandlers...)
		publicGroup.DELETE("/:id/photos/:mediaId", deletePhotoHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

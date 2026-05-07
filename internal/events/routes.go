package events

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, es EventServicePort, protected ...gin.HandlerFunc) {
	controller := &EventController{EventService: es}

	eventGroup := r.Group("/api/events")
	{
		eventGroup.GET("/locations", controller.ListSavedLocations)
		eventGroup.GET("/galleries", controller.ListGalleries)
		eventGroup.GET("", controller.ListEvents)
		eventGroup.GET("/:id", controller.GetEvent)
		eventGroup.POST("", controller.CreateEvent)
		eventGroup.PUT("/:id", controller.UpdateEvent)
		eventGroup.DELETE("/:id", controller.DeleteEvent)
		eventGroup.DELETE("/:id/documents/:mediaId", controller.DeleteEventDocument)
		eventGroup.DELETE("/:id/documents", controller.DeleteAllEventDocuments)
		eventGroup.DELETE("/:id/photos/:mediaId", controller.DeleteEventPhoto)
		if len(protected) > 0 {
			handlers := append([]gin.HandlerFunc{}, protected...)
			handlers = append(handlers, controller.GetEventMediaContent)
			eventGroup.GET("/:id/media/:mediaId/content", handlers...)
		}
	}
}

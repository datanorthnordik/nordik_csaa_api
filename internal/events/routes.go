package events

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, es EventServicePort) {
	controller := &EventController{EventService: es}

	publicGroup := r.Group("/api/events")
	{
		publicGroup.GET("", controller.ListEvents)
		publicGroup.GET("/:id", controller.GetEvent)
		publicGroup.GET("/:id/media/:mediaId/content", controller.GetEventMediaContent)
	}

	cmsGroup := r.Group("/api/cms/events")
	{
		cmsGroup.GET("/locations", controller.ListSavedLocations)
		cmsGroup.GET("/galleries", controller.ListGalleries)
		cmsGroup.POST("", controller.CreateEvent)
		cmsGroup.PUT("/:id", controller.UpdateEvent)
		cmsGroup.DELETE("/:id", controller.DeleteEvent)
		cmsGroup.DELETE("/:id/documents/:mediaId", controller.DeleteEventDocument)
		cmsGroup.DELETE("/:id/documents", controller.DeleteAllEventDocuments)
		cmsGroup.DELETE("/:id/photos/:mediaId", controller.DeleteEventPhoto)
	}
}

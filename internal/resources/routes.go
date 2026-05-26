package resources

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, rs ResourceServicePort, protected ...gin.HandlerFunc) {
	controller := &ResourceController{ResourceService: rs}

	publicGroup := r.Group("/api/resources")
	{
		postHandlers := withProtected(controller.CreateResource, protected...)
		putHandlers := withProtected(controller.UpdateResource, protected...)
		deleteHandlers := withProtected(controller.DeleteResource, protected...)

		publicGroup.GET("", controller.ListResources)
		publicGroup.GET("/:id", controller.GetResource)
		publicGroup.GET("/:id/content", controller.GetResourceContent)
		publicGroup.POST("", postHandlers...)
		publicGroup.PUT("/:id", putHandlers...)
		publicGroup.DELETE("/:id", deleteHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

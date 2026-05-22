package resources

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, rs ResourceServicePort, protected ...gin.HandlerFunc) {
	controller := &ResourceController{ResourceService: rs}

	group := r.Group("/api/resources")
	{
		listHandlers := withProtected(controller.ListResources, protected...)
		getHandlers := withProtected(controller.GetResource, protected...)
		contentHandlers := withProtected(controller.GetResourceContent, protected...)
		postHandlers := withProtected(controller.CreateResource, protected...)
		putHandlers := withProtected(controller.UpdateResource, protected...)
		deleteHandlers := withProtected(controller.DeleteResource, protected...)

		group.GET("", listHandlers...)
		group.GET("/:id", getHandlers...)
		group.GET("/:id/content", contentHandlers...)
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

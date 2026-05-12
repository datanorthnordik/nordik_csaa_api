package menus

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, ms MenuServicePort, protected ...gin.HandlerFunc) {
	controller := &MenuController{MenuService: ms}

	publicGroup := r.Group("/api/menus")
	{
		getPageOptionsHandlers := withProtected(controller.ListMenuPageOptions, protected...)
		saveHandlers := withProtected(controller.SaveMenu, protected...)

		publicGroup.GET("/:key", controller.GetMenu)
		publicGroup.GET("/:key/page-options", getPageOptionsHandlers...)
		publicGroup.PUT("/:key", saveHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

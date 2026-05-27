package pages

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, ps PageServicePort, protected ...gin.HandlerFunc) {
	controller := &PageController{PageService: ps}

	publicGroup := r.Group("/api/pages")
	{
		publicGroup.GET("", controller.ListPages)
		publicGroup.GET("/by-slug", controller.GetPageBySlug)
		publicGroup.GET("/sections/:sectionId/cta-image/content", controller.GetPageCTABannerImageContent)
		publicGroup.GET("/documents/:documentId/content", controller.GetPageDocumentContent)
		publicGroup.GET("/:id", controller.GetPage)
		publicGroup.GET("/:id/hero/content", controller.GetPageHeroImageContent)

		postHandlers := withProtected(controller.CreatePage, protected...)
		putHandlers := withProtected(controller.UpdatePage, protected...)
		deleteHandlers := withProtected(controller.DeletePage, protected...)

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

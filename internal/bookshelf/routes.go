package bookshelf

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, bs BookshelfServicePort, protected ...gin.HandlerFunc) {
	controller := &BookshelfController{BookshelfService: bs}

	group := r.Group("/api/bookshelf")
	{
		postHandlers := withProtected(controller.CreateBook, protected...)
		putHandlers := withProtected(controller.UpdateBook, protected...)
		deleteHandlers := withProtected(controller.DeleteBook, protected...)

		group.GET("", controller.ListBooks)
		group.GET("/:id", controller.GetBook)
		group.GET("/:id/book/content", controller.GetBookContent)
		group.GET("/:id/author-image/content", controller.GetAuthorImageContent)
		group.GET("/:id/cover/content", controller.GetCoverImageContent)
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

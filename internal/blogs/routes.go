package blogs

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, bs BlogServicePort, protected ...gin.HandlerFunc) {
	controller := &BlogController{BlogService: bs}

	publicGroup := r.Group("/api/blogs")
	{
		publicGroup.GET("", controller.ListBlogs)
		publicGroup.GET("/:id", controller.GetBlog)
		publicGroup.GET("/:id/cover/content", controller.GetBlogCoverImageContent)
		publicGroup.GET("/:id/sections/:sectionId/image/content", controller.GetBlogSectionImageContent)
		publicGroup.GET("/:id/sections/:sectionId/items/:itemId/image/content", controller.GetBlogAnimationItemImageContent)

		postHandlers := withProtected(controller.CreateBlog, protected...)
		putHandlers := withProtected(controller.UpdateBlog, protected...)
		deleteHandlers := withProtected(controller.DeleteBlog, protected...)

		publicGroup.POST("", postHandlers...)
		publicGroup.PUT("/:id", putHandlers...)
		publicGroup.DELETE("/:id", deleteHandlers...)
	}
}

package books

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, bs BookServicePort, protected ...gin.HandlerFunc) {
	controller := &BookController{BookService: bs}

	publicGroup := r.Group("/api/books")
	{
		publicGroup.GET("/public", controller.ListPublicBooks)
		publicGroup.GET("/public/:bookId", controller.GetPublicBook)
		publicGroup.GET("/public/:bookId/pdf/content", controller.GetPublicActivePDFContent)
		publicGroup.POST("/public/:bookId/submissions", controller.CreatePublicSubmission)

		getHandlers := withProtected(controller.ListBooks, protected...)
		getBookHandlers := withProtected(controller.GetBook, protected...)
		createHandlers := withProtected(controller.CreateBook, protected...)
		updateHandlers := withProtected(controller.UpdateBook, protected...)
		createVersionHandlers := withProtected(controller.CreateBookVersion, protected...)
		updateVersionHandlers := withProtected(controller.UpdateBookVersion, protected...)
		activateHandlers := withProtected(controller.SetActiveVersion, protected...)
		versionDetailHandlers := withProtected(controller.GetBookVersionDetail, protected...)
		uploadGeneratedHandlers := withProtected(controller.UploadGeneratedPDF, protected...)
		sourceContentHandlers := withProtected(controller.GetSourcePDFContent, protected...)
		generatedContentHandlers := withProtected(controller.GetGeneratedPDFContent, protected...)
		submissionImageHandlers := withProtected(controller.GetSubmissionImageContent, protected...)
		listSubmissionHandlers := withProtected(controller.ListBookSubmissions, protected...)
		updateSubmissionHandlers := withProtected(controller.UpdateBookSubmission, protected...)
		approveSubmissionHandlers := withProtected(controller.ApproveBookSubmission, protected...)
		rejectSubmissionHandlers := withProtected(controller.RejectBookSubmission, protected...)

		publicGroup.GET("", getHandlers...)
		publicGroup.GET("/:bookId", getBookHandlers...)
		publicGroup.POST("", createHandlers...)
		publicGroup.PUT("/:bookId", updateHandlers...)
		publicGroup.POST("/:bookId/versions", createVersionHandlers...)
		publicGroup.PUT("/:bookId/versions/:versionId", updateVersionHandlers...)
		publicGroup.POST("/:bookId/versions/:versionId/activate", activateHandlers...)
		publicGroup.GET("/:bookId/versions/:versionId", versionDetailHandlers...)
		publicGroup.PUT("/:bookId/versions/:versionId/generated", uploadGeneratedHandlers...)
		publicGroup.GET("/:bookId/versions/:versionId/source/content", sourceContentHandlers...)
		publicGroup.GET("/:bookId/versions/:versionId/generated/content", generatedContentHandlers...)
		publicGroup.GET("/:bookId/submissions", listSubmissionHandlers...)
		publicGroup.PUT("/:bookId/submissions/:submissionId", updateSubmissionHandlers...)
		publicGroup.POST("/:bookId/submissions/:submissionId/approve", approveSubmissionHandlers...)
		publicGroup.POST("/:bookId/submissions/:submissionId/reject", rejectSubmissionHandlers...)
		publicGroup.GET("/:bookId/submissions/:submissionId/image/content", submissionImageHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

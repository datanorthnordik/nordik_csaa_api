package knowledgecenter

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, service KnowledgeCenterServicePort, protected ...gin.HandlerFunc) {
	controller := &KnowledgeCenterController{KnowledgeCenterService: service}

	group := r.Group("/api/knowledge-center")
	{
		group.POST("/submissions", controller.CreatePublicSubmission)

		listHandlers := withProtected(controller.ListSubmissions, protected...)
		getHandlers := withProtected(controller.GetSubmission, protected...)
		completeHandlers := withProtected(controller.MarkSubmissionCompleted, protected...)

		group.GET("/submissions", listHandlers...)
		group.GET("/submissions/:submissionId", getHandlers...)
		group.POST("/submissions/:submissionId/complete", completeHandlers...)
	}
}

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

package latestcontent

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, service ServicePort) {
	controller := &Controller{Service: service}
	r.GET("/api/latest-content", controller.ListLatestContent)
}

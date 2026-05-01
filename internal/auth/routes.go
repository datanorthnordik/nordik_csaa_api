package auth

import (
	"nordikcsaaapi/internal/config"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, as AuthServicePort, cfg *config.Config) {
	controller := &AuthController{AuthService: as, CFG: cfg}

	userGroup := r.Group("/api/user")
	{
		userGroup.POST("/login", controller.Login)
		userGroup.POST("/signup", controller.SignUp)
		userGroup.POST("/refresh", controller.Refresh)
	}
}

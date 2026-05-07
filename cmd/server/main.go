package main

import (
	"log"
	"net/http"
	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"
	"nordikcsaaapi/internal/events"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.LoadConfig()

	dsn := cfg.DatabaseURL
	if dsn == "" {
		dbPort := cfg.DBPort
		if dbPort == "" {
			dbPort = "5432"
		}

		sslMode := cfg.DBSSLMode
		if sslMode == "" {
			sslMode = "disable"
		}

		dsn = "host=" + cfg.DBHost +
			" user=" + cfg.DBUser +
			" password=" + cfg.DBPassword +
			" dbname=" + cfg.DBName +
			" port=" + dbPort +
			" sslmode=" + sslMode
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("panic recovered while handling %s %s: %v", c.Request.Method, c.Request.URL.Path, recovered)
		apiresponse.WriteInternalError(c)
	}))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	r.NoRoute(apiresponse.WriteRouteNotFound)
	r.NoMethod(apiresponse.WriteMethodNotAllowed)

	userService := &auth.AuthService{DB: db}
	auth.RegisterRoutes(r, userService, &cfg)
	eventService := &events.EventService{DB: db, BucketName: cfg.DriveBucketName}
	events.RegisterRoutes(r, eventService, auth.RequireBearerAuth(&cfg))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on 0.0.0.0:%s ...", port)
	log.Fatal(r.Run("0.0.0.0:" + port))
}

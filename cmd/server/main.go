package main

import (
	"log"
	"net/http"
	"nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"
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

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	userService := &auth.AuthService{DB: db}
	auth.RegisterRoutes(r, userService, &cfg)
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

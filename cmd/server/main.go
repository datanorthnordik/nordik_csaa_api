package main

import (
	"log"
	"net/http"
	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/auth"
	"nordikcsaaapi/internal/config"
	"nordikcsaaapi/internal/events"
	"nordikcsaaapi/internal/gallery"
	"nordikcsaaapi/internal/memorial"
	"nordikcsaaapi/internal/menus"
	"nordikcsaaapi/internal/newsletters"
	"nordikcsaaapi/internal/pages"
	"nordikcsaaapi/internal/press"
	"nordikcsaaapi/internal/resources"
	"nordikcsaaapi/internal/video"
	"os"
	"time"

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
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to initialize database pool:", err)
	}
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("panic recovered while handling %s %s: %v", c.Request.Method, c.Request.URL.Path, recovered)
		apiresponse.WriteInternalError(c)
	}))
	r.Use(cors.New(buildCORSConfig()))
	r.NoRoute(apiresponse.WriteRouteNotFound)
	r.NoMethod(apiresponse.WriteMethodNotAllowed)

	userService := &auth.AuthService{DB: db}
	auth.RegisterRoutes(r, userService, &cfg)
	eventService := &events.EventService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	events.RegisterRoutes(r, eventService, auth.RequireBearerAuth(&cfg))
	galleryService := &gallery.GalleryService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	gallery.RegisterRoutes(r, galleryService, auth.RequireBearerAuth(&cfg))
	videoService := &video.VideoService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	video.RegisterRoutes(r, videoService, auth.RequireBearerAuth(&cfg))
	pageService := &pages.PageService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	pages.RegisterRoutes(r, pageService, auth.RequireBearerAuth(&cfg))
	menuService := &menus.MenuService{DB: db}
	menus.RegisterRoutes(r, menuService, auth.RequireBearerAuth(&cfg))
	newsletterService := &newsletters.NewsletterService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	newsletters.RegisterRoutes(r, newsletterService, auth.RequireBearerAuth(&cfg))
	pressService := &press.PressService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	press.RegisterRoutes(r, pressService, auth.RequireBearerAuth(&cfg))
	resourceService := &resources.ResourceService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	resources.RegisterRoutes(r, resourceService, auth.RequireBearerAuth(&cfg))
	memorialService := &memorial.MemorialService{DB: db, BucketName: cfg.DriveBucketName, BucketPrefix: cfg.DriveBucketPrefix}
	memorial.RegisterRoutes(r, memorialService, auth.RequireBearerAuth(&cfg))
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

func buildCORSConfig() cors.Config {
	return cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"https://nordikcsaacms-724838782318.us-west1.run.app",
			"https://nordikcsaawebsite-724838782318.us-west1.run.app",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}
}

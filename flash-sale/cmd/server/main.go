package main

import (
	"flash-sale/internal/middleware"
	"flash-sale/internal/user"
	"flash-sale/pkg/config"
	"flash-sale/pkg/database"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// connect to database
	db, err := database.NewPostgres(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// auto migrate
	if err := db.AutoMigrate(&user.User{}); err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	// init layers
	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo, cfg.JWT.Secret, cfg.JWT.ExpireHours)
	userHandler := user.NewHandler(userService)

	// setup router
	r := gin.New()

	// custom logger with instance port
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] [INFO] [instance:%d] %s %s %d %s\n",
			param.TimeStamp.Format(time.RFC3339),
			cfg.Server.Port,
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
		)
	}))
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")
	publicGroup := api.Group("")
	authGroup := api.Group("")
	authGroup.Use(middleware.JWTAuth(cfg.JWT.Secret))

	// register routes
	userHandler.RegisterRoutes(publicGroup, authGroup)

	// health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

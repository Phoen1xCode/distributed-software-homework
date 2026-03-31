package main

import (
	"flash-sale/internal/inventory"
	"flash-sale/internal/middleware"
	"flash-sale/internal/order"
	"flash-sale/internal/product"
	"flash-sale/internal/user"
	"flash-sale/pkg/cache"
	"flash-sale/pkg/config"
	"flash-sale/pkg/database"
	"flash-sale/pkg/snowflake"
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

	// connect to redis
	redisClient := cache.NewRedisClient(cfg.Redis)
	defer redisClient.Close()

	// auto migrate
	if err := db.AutoMigrate(&user.User{}, &product.Product{}, &inventory.Inventory{}, &order.Order{}); err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	// user module
	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo, cfg.JWT.Secret, cfg.JWT.ExpireHours)
	userHandler := user.NewHandler(userService)

	// inventory module
	inventoryRepo := inventory.NewRepository(db)
	inventoryService := inventory.NewService(inventoryRepo)
	inventoryHandler := inventory.NewHandler(inventoryService)

	// product module
	productRepo := product.NewRepository(db)
	productService := product.NewService(productRepo)
	cachedProductService := product.NewCachedService(productService, redisClient)
	productHandler := product.NewHandler(cachedProductService, inventoryService)

	// snowflake node
	sfNode, err := snowflake.NewNode(cfg.Snowflake.NodeID)
	if err != nil {
		log.Fatalf("Failed to create snowflake node: %v", err)
	}

	// order module
	orderRepo := order.NewRepository(db)
	orderService := order.NewService(orderRepo, productService, inventoryService, sfNode, redisClient)
	orderHandler := order.NewHandler(orderService)
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
	// public routes
	publicGroup := api.Group("")
	// auth routes
	authGroup := api.Group("")
	authGroup.Use(middleware.JWTAuth(cfg.JWT.Secret))
	// admin routes
	adminGroup := api.Group("")
	adminGroup.Use(middleware.JWTAuth(cfg.JWT.Secret))
	adminGroup.Use(middleware.AdminRequired())

	// register routes
	userHandler.RegisterRoutes(publicGroup, authGroup)
	productHandler.RegisterRoutes(publicGroup, adminGroup)
	inventoryHandler.RegisterRoutes(publicGroup, authGroup, adminGroup)
	orderHandler.RegisterRoutes(authGroup)

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

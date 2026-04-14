package main

import (
	"context"
	"flash-sale/internal/inventory"
	"flash-sale/internal/middleware"
	"flash-sale/internal/order"
	"flash-sale/internal/product"
	"flash-sale/internal/user"
	"flash-sale/pkg/cache"
	"flash-sale/pkg/config"
	"flash-sale/pkg/database"
	"flash-sale/pkg/kafka"
	"flash-sale/pkg/outbox"
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

	db, err := database.NewPostgresWithReplicas(cfg.Database.DSN(), cfg.Database.ReplicaDSNs())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	redisClient := cache.NewRedisClient(cfg.Redis)
	defer redisClient.Close()

	// Auto migrate order-service tables
	if err := db.AutoMigrate(&user.User{}, &product.Product{}, &order.Order{}, &outbox.OutboxEvent{}); err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	// User module
	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo, cfg.JWT.Secret, cfg.JWT.ExpireHours)
	userHandler := user.NewHandler(userService)

	// Product module (read context for orders)
	productRepo := product.NewRepository(db)
	productService := product.NewService(productRepo)
	cachedProductService := product.NewCachedService(productService, redisClient)
	// Inventory service stub for product handler (read-only, not the real inventory DB)
	inventoryRepo := inventory.NewRepository(db)
	inventoryService := inventory.NewService(inventoryRepo)
	productHandler := product.NewHandler(cachedProductService, inventoryService)

	// Snowflake
	sfNode, err := snowflake.NewNode(cfg.Snowflake.NodeID)
	if err != nil {
		log.Fatalf("Failed to create snowflake node: %v", err)
	}

	// Order module
	orderRepo := order.NewRepository(db)
	orderService := order.NewService(orderRepo, productService, inventoryService, sfNode, redisClient)
	orderHandler := order.NewHandler(orderService)

	// Seckill module
	seckillService := order.NewSeckillService(orderRepo, db, productService, inventoryService, sfNode, redisClient)
	seckillHandler := order.NewSeckillHandler(seckillService)

	// Kafka producer for outbox relay
	kafkaProducer, err := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ProduceTopic)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	// Outbox relay
	interval := 500 * time.Millisecond
	if cfg.Outbox.IntervalMs > 0 {
		interval = time.Duration(cfg.Outbox.IntervalMs) * time.Millisecond
	}
	relay := outbox.NewRelay(db, kafkaProducer, interval)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go relay.Start(ctx)

	// Order event handler (consumes inventory-events and payment-events)
	orderEventHandler := order.NewEventHandler(orderRepo, db, redisClient)
	for _, cg := range cfg.Kafka.ConsumerGroups {
		var handler kafka.MessageHandler
		switch cg.Topic {
		case "inventory-events":
			handler = orderEventHandler.HandleInventoryEvent
		case "payment-events":
			handler = orderEventHandler.HandlePaymentEvent
		default:
			log.Printf("[WARN] Unknown consumer topic: %s", cg.Topic)
			continue
		}
		consumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, cg.GroupID, cg.Topic, handler)
		if err != nil {
			log.Printf("[WARN] Failed to create consumer for %s: %v", cg.Topic, err)
			continue
		}
		go consumer.Start(ctx)
		defer consumer.Close()
	}

	// Router
	r := gin.New()
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] [INFO] [order-service:%d] %s %s %d %s\n",
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
	adminGroup := api.Group("")
	adminGroup.Use(middleware.JWTAuth(cfg.JWT.Secret))
	adminGroup.Use(middleware.AdminRequired())

	userHandler.RegisterRoutes(publicGroup, authGroup)
	productHandler.RegisterRoutes(publicGroup, adminGroup)
	orderHandler.RegisterRoutes(authGroup)
	seckillHandler.RegisterRoutes(authGroup)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "order-service"})
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Order service starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

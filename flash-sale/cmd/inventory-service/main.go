package main

import (
	"context"
	"flash-sale/internal/inventory"
	"flash-sale/internal/middleware"
	"flash-sale/pkg/cache"
	"flash-sale/pkg/config"
	"flash-sale/pkg/database"
	"flash-sale/pkg/kafka"
	"flash-sale/pkg/outbox"
	"flash-sale/pkg/registry"
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

	db, err := database.NewPostgres(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	redisClient := cache.NewRedisClient(cfg.Redis)
	defer redisClient.Close()

	if err := db.AutoMigrate(&inventory.Inventory{}, &outbox.OutboxEvent{}); err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	inventoryRepo := inventory.NewRepository(db)
	inventoryService := inventory.NewService(inventoryRepo)
	inventoryHandler := inventory.NewHandler(inventoryService)

	// Kafka producer for outbox
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

	// Event handler
	eventHandler := inventory.NewEventHandler(inventoryService, db, redisClient)
	for _, cg := range cfg.Kafka.ConsumerGroups {
		consumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, cg.GroupID, cg.Topic, eventHandler.HandleOrderEvent)
		if err != nil {
			log.Printf("[WARN] Failed to create consumer for %s: %v", cg.Topic, err)
			continue
		}
		go consumer.Start(ctx)
		defer consumer.Close()
	}

	// Router (inventory HTTP API)
	r := gin.New()
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] [INFO] [inventory-service:%d] %s %s %d %s\n",
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

	inventoryHandler.RegisterRoutes(publicGroup, authGroup, adminGroup)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "inventory-service"})
	})

	_, deregister := registry.BootstrapService(cfg)
	defer deregister()

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Inventory service starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

package main

import (
	"context"
	"flash-sale/internal/payment"
	"flash-sale/pkg/config"
	"flash-sale/pkg/database"
	"flash-sale/pkg/kafka"
	"flash-sale/pkg/outbox"
	"flash-sale/pkg/registry"
	"fmt"
	"log"
	"strconv"
	"strings"
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

	if err := db.AutoMigrate(&payment.Payment{}, &outbox.OutboxEvent{}); err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	successRate := cfg.Payment.SuccessRate
	if successRate == 0 {
		successRate = 1.0
	}
	paymentService := payment.NewService(db, successRate)

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

	// Consume order-events for PAYMENT_REQUESTED
	for _, cg := range cfg.Kafka.ConsumerGroups {
		consumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, cg.GroupID, cg.Topic, paymentService.HandleOrderEvent)
		if err != nil {
			log.Printf("[WARN] Failed to create consumer for %s: %v", cg.Topic, err)
			continue
		}
		go consumer.Start(ctx)
		defer consumer.Close()
	}

	// Minimal HTTP server for health check
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "payment-service"})
	})

	nacosClient, deregister := registry.BootstrapService(cfg)
	defer deregister()

	// Dynamic config: subscribe payment.success_rate from Nacos.
	// The DataID defaults to the service name when not explicitly set so the
	// Nacos console operator can target this service unambiguously.
	if nacosClient != nil {
		dataID := cfg.Nacos.DataID
		if dataID == "" {
			dataID = "payment-service.yaml"
		}
		if err := nacosClient.ListenConfig(dataID, func(content string) {
			rate, perr := strconv.ParseFloat(strings.TrimSpace(content), 64)
			if perr != nil {
				log.Printf("[payment] dynamic config %s: invalid float %q: %v", dataID, content, perr)
				return
			}
			paymentService.SetSuccessRate(rate)
			log.Printf("[payment] dynamic config %s -> success_rate=%.3f", dataID, rate)
		}); err != nil {
			log.Printf("[payment] listen config %s failed: %v", dataID, err)
		}
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Payment service starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

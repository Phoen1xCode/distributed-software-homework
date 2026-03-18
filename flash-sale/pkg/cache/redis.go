package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"flash-sale/pkg/config"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg config.RedisConfig) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[WARN] Redis connection failed: %v (cache disabled, falling back to DB)", err)
	}

	return client
}

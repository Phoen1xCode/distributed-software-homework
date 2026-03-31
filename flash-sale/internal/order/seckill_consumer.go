package order

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"flash-sale/internal/inventory"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type SeckillConsumer struct {
	repo             *Repository
	inventoryService *inventory.InventoryService
	rdb              *redis.Client
}

func NewSeckillConsumer(repo *Repository, inventorySvc *inventory.InventoryService, rdb *redis.Client) *SeckillConsumer {
	return &SeckillConsumer{
		repo:             repo,
		inventoryService: inventorySvc,
		rdb:              rdb,
	}
}

func (c *SeckillConsumer) HandleMessage(key, value []byte) error {
	var msg SeckillMessage
	if err := json.Unmarshal(value, &msg); err != nil {
		log.Printf("[ERROR] Failed to unmarshal seckill message: %v", err)
		return nil
	}

	log.Printf("[INFO] Processing seckill order: %s, user: %d, product: %d", msg.OrderNo, msg.UserID, msg.ProductID)

	resultKey := fmt.Sprintf("seckill:result:%s", msg.OrderNo)

	o := &Order{
		OrderNo:    msg.OrderNo,
		UserID:     msg.UserID,
		ProductID:  msg.ProductID,
		Quantity:   msg.Quantity,
		TotalPrice: msg.Price,
		Status:     0,
	}

	err := c.repo.Transaction(func(tx *gorm.DB) error {
		txInventory := c.inventoryService.WithDB(tx)
		if err := txInventory.Deduct(msg.ProductID, msg.Quantity); err != nil {
			return err
		}
		return c.repo.CreateTx(tx, o)
	})

	if err != nil {
		log.Printf("[ERROR] Seckill order %s failed: %v", msg.OrderNo, err)
		inventory.RollbackRedis(c.rdb, msg.ProductID, msg.Quantity)
		idempotencyKey := fmt.Sprintf("seckill:lock:%d:%d", msg.UserID, msg.ProductID)
		c.rdb.Del(context.Background(), idempotencyKey)
		c.rdb.Set(context.Background(), resultKey, "FAILED", 0)
		return nil
	}

	c.rdb.Set(context.Background(), resultKey, "SUCCESS", 0)
	log.Printf("[INFO] Seckill order %s created successfully", msg.OrderNo)
	return nil
}

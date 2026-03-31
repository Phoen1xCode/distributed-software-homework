package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"flash-sale/internal/inventory"
	"flash-sale/internal/product"
	"flash-sale/pkg/kafka"
	"flash-sale/pkg/snowflake"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type SeckillMessage struct {
	OrderNo   string  `json:"order_no"`
	UserID    uint    `json:"user_id"`
	ProductID uint    `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type SeckillService struct {
	repo             *Repository
	productService   *product.ProductService
	inventoryService *inventory.InventoryService
	snowflakeNode    *snowflake.Node
	rdb              *redis.Client
	producer         *kafka.Producer
}

func NewSeckillService(
	repo *Repository,
	productSvc *product.ProductService,
	inventorySvc *inventory.InventoryService,
	sfNode *snowflake.Node,
	rdb *redis.Client,
	producer *kafka.Producer,
) *SeckillService {
	return &SeckillService{
		repo:             repo,
		productService:   productSvc,
		inventoryService: inventorySvc,
		snowflakeNode:    sfNode,
		rdb:              rdb,
		producer:         producer,
	}
}

func (s *SeckillService) Seckill(userID uint, req *CreateOrderRequest) (string, error) {
	ctx := context.Background()

	// 1. Idempotency check
	idempotencyKey := fmt.Sprintf("seckill:lock:%d:%d", userID, req.ProductID)
	set, err := s.rdb.SetNX(ctx, idempotencyKey, "1", 24*time.Hour).Result()
	if err != nil {
		log.Printf("[WARN] Redis SETNX failed: %v, falling back to DB", err)
		exists, dbErr := s.repo.ExistsByUserAndProduct(userID, req.ProductID)
		if dbErr != nil {
			return "", dbErr
		}
		if exists {
			return "", ErrDuplicateOrder
		}
	} else if !set {
		return "", ErrDuplicateOrder
	}

	// 2. Check product exists and is on sale
	p, err := s.productService.GetProductByID(req.ProductID)
	if err != nil {
		s.rdb.Del(ctx, idempotencyKey)
		if errors.Is(err, product.ErrProductNotFound) {
			return "", ErrProductNotFound
		}
		return "", err
	}
	if p.Status != 1 {
		s.rdb.Del(ctx, idempotencyKey)
		return "", ErrProductOffSale
	}

	// 3. Redis stock deduction (fast path)
	if err := inventory.DeductRedis(s.rdb, req.ProductID, req.Quantity); err != nil {
		s.rdb.Del(ctx, idempotencyKey)
		return "", ErrStockInsufficient
	}

	// 4. Generate order number and send to Kafka
	orderNo := s.snowflakeNode.GenerateString()
	msg := SeckillMessage{
		OrderNo:   orderNo,
		UserID:    userID,
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
		Price:     p.Price * float64(req.Quantity),
	}
	data, _ := json.Marshal(msg)

	if err := s.producer.SendMessage([]byte(orderNo), data); err != nil {
		inventory.RollbackRedis(s.rdb, req.ProductID, req.Quantity)
		s.rdb.Del(ctx, idempotencyKey)
		log.Printf("[ERROR] Kafka send failed: %v", err)
		return "", fmt.Errorf("failed to enqueue order: %w", err)
	}

	// 5. Store pending status in Redis
	s.rdb.Set(ctx, fmt.Sprintf("seckill:result:%s", orderNo), "PENDING", 30*time.Minute)

	return orderNo, nil
}

func (s *SeckillService) GetSeckillResult(orderNo string) (string, error) {
	ctx := context.Background()

	status, err := s.rdb.Get(ctx, fmt.Sprintf("seckill:result:%s", orderNo)).Result()
	if err == nil {
		return status, nil
	}

	var o Order
	if err := s.repo.db.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "PENDING", nil
		}
		return "", err
	}
	return "SUCCESS", nil
}

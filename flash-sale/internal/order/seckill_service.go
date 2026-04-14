package order

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"flash-sale/internal/inventory"
	"flash-sale/internal/product"
	"flash-sale/pkg/event"
	"flash-sale/pkg/outbox"
	"flash-sale/pkg/snowflake"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type SeckillService struct {
	repo             *Repository
	db               *gorm.DB
	productService   *product.ProductService
	inventoryService *inventory.InventoryService
	snowflakeNode    *snowflake.Node
	rdb              *redis.Client
}

func NewSeckillService(
	repo *Repository,
	db *gorm.DB,
	productSvc *product.ProductService,
	inventorySvc *inventory.InventoryService,
	sfNode *snowflake.Node,
	rdb *redis.Client,
) *SeckillService {
	return &SeckillService{
		repo:             repo,
		db:               db,
		productService:   productSvc,
		inventoryService: inventorySvc,
		snowflakeNode:    sfNode,
		rdb:              rdb,
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

	// 4. Create order + outbox event in same transaction
	orderNo := s.snowflakeNode.GenerateString()
	totalPrice := p.Price * float64(req.Quantity)

	o := &Order{
		OrderNo:    orderNo,
		UserID:     userID,
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		TotalPrice: totalPrice,
		Status:     StatusPending,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(o).Error; err != nil {
			return err
		}
		return outbox.WriteEvent(tx, "order", orderNo, event.OrderCreated, event.TopicOrderEvents,
			event.OrderCreatedEvent{
				BaseEvent: event.BaseEvent{
					EventID:   uuid.New().String(),
					EventType: event.OrderCreated,
					Timestamp: time.Now(),
				},
				OrderNo:    orderNo,
				UserID:     userID,
				ProductID:  req.ProductID,
				Quantity:   req.Quantity,
				TotalPrice: totalPrice,
			})
	})
	if err != nil {
		// Rollback Redis on DB failure
		inventory.RollbackRedis(s.rdb, req.ProductID, req.Quantity)
		s.rdb.Del(ctx, idempotencyKey)
		log.Printf("[ERROR] Seckill DB transaction failed: %v", err)
		return "", fmt.Errorf("failed to create order: %w", err)
	}

	// 5. Store pending status in Redis for polling
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

	switch o.Status {
	case StatusPaid:
		return "SUCCESS", nil
	case StatusCancelled:
		return "FAILED", nil
	default:
		return "PENDING", nil
	}
}

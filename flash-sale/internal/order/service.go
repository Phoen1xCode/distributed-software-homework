package order

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"flash-sale/internal/inventory"
	"flash-sale/internal/product"
	"flash-sale/pkg/snowflake"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrOrderNotPending   = errors.New("order is not in pending status")
	ErrNotOrderOwner     = errors.New("not the owner of this order")
	ErrProductNotFound   = errors.New("product not found")
	ErrProductOffSale    = errors.New("product is not on sale")
	ErrStockInsufficient = errors.New("stock insufficient")
	ErrDuplicateOrder    = errors.New("you have already purchased this product")
)

type OrderService struct {
	repo             *Repository
	productService   *product.ProductService
	inventoryService *inventory.InventoryService
	snowflakeNode    *snowflake.Node
	rdb              *redis.Client
}

func NewService(repo *Repository, productSvc *product.ProductService, inventorySvc *inventory.InventoryService, sfNode *snowflake.Node, rdb *redis.Client) *OrderService {
	return &OrderService{
		repo:             repo,
		productService:   productSvc,
		inventoryService: inventorySvc,
		snowflakeNode:    sfNode,
		rdb:              rdb,
	}
}

func (s *OrderService) CreateOrder(userID uint, req *CreateOrderRequest) (*Order, error) {
	// Idempotency check: one user can only buy each product once
	idempotencyKey := fmt.Sprintf("seckill:lock:%d:%d", userID, req.ProductID)
	ctx := context.Background()

	set, err := s.rdb.SetNX(ctx, idempotencyKey, "1", 24*time.Hour).Result()
	if err != nil {
		log.Printf("[WARN] Redis SETNX failed: %v, falling back to DB", err)
		exists, dbErr := s.repo.ExistsByUserAndProduct(userID, req.ProductID)
		if dbErr != nil {
			return nil, dbErr
		}
		if exists {
			return nil, ErrDuplicateOrder
		}
	} else if !set {
		return nil, ErrDuplicateOrder
	}

	// 1. Check product exists and is on sale (read-only, outside transaction)
	p, err := s.productService.GetProductByID(req.ProductID)
	if err != nil {
		if errors.Is(err, product.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	if p.Status != 1 {
		return nil, ErrProductOffSale
	}

	// 2. Deduct inventory and create order atomically in one transaction
	o := &Order{
		OrderNo:    s.snowflakeNode.GenerateString(),
		UserID:     userID,
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		TotalPrice: p.Price * float64(req.Quantity),
		Status:     0,
	}

	err = s.repo.Transaction(func(tx *gorm.DB) error {
		txInventory := s.inventoryService.WithDB(tx)
		if err := txInventory.Deduct(req.ProductID, req.Quantity); err != nil {
			return err
		}
		if err := s.repo.CreateTx(tx, o); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// Rollback idempotency key on failure
		if delErr := s.rdb.Del(ctx, idempotencyKey).Err(); delErr != nil {
			log.Printf("[WARN] Redis DEL idempotency key failed: %v", delErr)
		}
		if errors.Is(err, inventory.ErrInventoryNotFound) || errors.Is(err, inventory.ErrStockInsufficient) {
			return nil, ErrStockInsufficient
		}
		return nil, err
	}

	return o, nil
}

func (s *OrderService) GetOrderByID(userID, orderID uint) (*Order, error) {
	o, err := s.repo.GetOrderByID(orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if o.UserID != userID {
		return nil, ErrNotOrderOwner
	}
	return o, nil
}

func (s *OrderService) ListOrderByUser(userID uint, page, pageSize int) ([]Order, int64, error) {
	return s.repo.GetOrderByUserID(userID, page, pageSize)
}

func (s *OrderService) CancelOrder(userID, orderID uint) (*Order, error) {
	var o *Order
	err := s.repo.Transaction(func(tx *gorm.DB) error {
		// Lock the order row to prevent concurrent cancellation
		var err error
		o, err = s.repo.GetOrderByIDForUpdate(tx, orderID)
		if err != nil {
			return err
		}
		if o.UserID != userID {
			return ErrNotOrderOwner
		}
		if o.Status != StatusPending {
			return ErrOrderNotPending
		}

		// Return inventory within the same transaction
		txInventory := s.inventoryService.WithDB(tx)
		if err := txInventory.Return(o.ProductID, o.Quantity); err != nil {
			return err
		}

		// Update order status to cancelled
		if err := s.repo.UpdateStatusTx(tx, orderID, StatusCancelled); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		if errors.Is(err, ErrNotOrderOwner) {
			return nil, ErrNotOrderOwner
		}
		if errors.Is(err, ErrOrderNotPending) {
			return nil, ErrOrderNotPending
		}
		return nil, err
	}

	o.Status = StatusCancelled
	return o, nil
}


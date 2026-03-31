package inventory

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const maxDeductRetries = 3

var (
	ErrInventoryNotFound = errors.New("inventory not found")
	ErrStockInsufficient = errors.New("stock insufficient")
)

type InventoryService struct {
	repo *Repository
}

func NewService(repo *Repository) *InventoryService {
	return &InventoryService{repo: repo}
}

// WithDB returns a copy of the service backed by the given *gorm.DB (e.g. a transaction).
func (s *InventoryService) WithDB(db *gorm.DB) *InventoryService {
	return &InventoryService{repo: NewRepository(db)}
}

func (s *InventoryService) GetByProductID(productID uint) (*Inventory, error) {
	inv, err := s.repo.GetByProductID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInventoryNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (s *InventoryService) SetInventory(productID uint, req *SetInventoryRequest) (*Inventory, error) {
	inv, err := s.repo.GetByProductID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			inv = &Inventory{
				ProductID: productID,
				Total:     req.Total,
				Available: req.Total,
				Locked:    0,
				Version:   0,
			}
			if err := s.repo.Upsert(inv); err != nil {
				return nil, err
			}
			return inv, nil
		}
		return nil, err
	}

	diff := req.Total - inv.Total
	inv.Total = req.Total
	inv.Available = inv.Available + diff
	if inv.Available < 0 {
		inv.Available = 0
	}

	if err := s.repo.Upsert(inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (s *InventoryService) Deduct(productID uint, quantity int) error {
	for i := 0; i < maxDeductRetries; i++ {
		inv, err := s.repo.GetByProductID(productID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInventoryNotFound
			}
			return err
		}

		if inv.Available < quantity {
			return ErrStockInsufficient
		}

		err = s.repo.DeductWithLock(productID, quantity, inv.Version)
		if err != nil {
			if errors.Is(err, ErrOptimisticLock) {
				continue // retry with fresh version
			}
			return err
		}
		return nil
	}
	return ErrStockInsufficient
}

func (s *InventoryService) Return(productID uint, quantity int) error {
	return s.repo.ReturnStock(productID, quantity)
}

const stockKeyPrefix = "inventory:stock:"

func stockKey(productID uint) string {
	return fmt.Sprintf("%s%d", stockKeyPrefix, productID)
}

// PreloadStock loads DB stock into Redis for seckill warmup.
func (s *InventoryService) PreloadStock(rdb *redis.Client, productID uint) error {
	inv, err := s.repo.GetByProductID(productID)
	if err != nil {
		return err
	}
	return rdb.Set(context.Background(), stockKey(productID), inv.Available, 0).Err()
}

// DeductRedis atomically decrements Redis stock. Returns error if insufficient.
func DeductRedis(rdb *redis.Client, productID uint, quantity int) error {
	key := stockKey(productID)
	result, err := rdb.DecrBy(context.Background(), key, int64(quantity)).Result()
	if err != nil {
		return fmt.Errorf("redis DECRBY failed: %w", err)
	}
	if result < 0 {
		rdb.IncrBy(context.Background(), key, int64(quantity))
		return ErrStockInsufficient
	}
	return nil
}

// RollbackRedis restores Redis stock on order creation failure.
func RollbackRedis(rdb *redis.Client, productID uint, quantity int) {
	if err := rdb.IncrBy(context.Background(), stockKey(productID), int64(quantity)).Err(); err != nil {
		log.Printf("[WARN] Redis stock rollback failed for product %d: %v", productID, err)
	}
}

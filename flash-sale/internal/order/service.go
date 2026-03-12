package order

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"flash-sale/internal/inventory"
	"flash-sale/internal/product"

	"gorm.io/gorm"
)

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrOrderNotPending   = errors.New("order is not in pending status")
	ErrNotOrderOwner     = errors.New("not the owner of this order")
	ErrProductNotFound   = errors.New("product not found")
	ErrProductOffSale    = errors.New("product is not on sale")
	ErrStockInsufficient = errors.New("stock insufficient")
)

type OrderService struct {
	repo             *Repository
	productService   *product.ProductService
	inventoryService *inventory.InventoryService
}

func NewService(repo *Repository, productSvc *product.ProductService, inventorySvc *inventory.InventoryService) *OrderService {
	return &OrderService{
		repo:             repo,
		productService:   productSvc,
		inventoryService: inventorySvc,
	}
}

func (s *OrderService) CreateOrder(userID uint, req *CreateOrderRequest) (*Order, error) {
	// 1. Check product exists and is on sale
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

	// 2. Deduct inventory (optimistic lock)
	if err := s.inventoryService.Deduct(req.ProductID, req.Quantity); err != nil {
		if errors.Is(err, inventory.ErrInventoryNotFound) {
			return nil, ErrStockInsufficient
		}
		if errors.Is(err, inventory.ErrStockInsufficient) {
			return nil, ErrStockInsufficient
		}
		return nil, err
	}

	// 3. Create order
	o := &Order{
		OrderNo:    generateOrderNo(),
		UserID:     userID,
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		TotalPrice: p.Price * float64(req.Quantity),
		Status:     0,
	}

	if err := s.repo.Create(o); err != nil {
		// Rollback: return inventory
		_ = s.inventoryService.Return(req.ProductID, req.Quantity)
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
	if o.Status != 0 {
		return nil, ErrOrderNotPending
	}

	// Return inventory
	if err := s.inventoryService.Return(o.ProductID, o.Quantity); err != nil {
		return nil, err
	}

	// Update order status to cancelled
	if err := s.repo.UpdateStatus(orderID, 2); err != nil {
		return nil, err
	}

	o.Status = 2
	return o, nil
}

func generateOrderNo() string {
	return fmt.Sprintf("%s%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

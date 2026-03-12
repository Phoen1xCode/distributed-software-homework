package order

import (
	"crypto/rand"
	"errors"
	"fmt"
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
		OrderNo:    generateOrderNo(),
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
		if o.Status != 0 {
			return ErrOrderNotPending
		}

		// Return inventory within the same transaction
		txInventory := s.inventoryService.WithDB(tx)
		if err := txInventory.Return(o.ProductID, o.Quantity); err != nil {
			return err
		}

		// Update order status to cancelled
		if err := s.repo.UpdateStatusTx(tx, orderID, 2); err != nil {
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

	o.Status = 2
	return o, nil
}

func generateOrderNo() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s%x", time.Now().Format("20060102150405"), b)
}

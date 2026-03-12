package inventory

import (
	"errors"

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

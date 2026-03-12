package inventory

import (
	"errors"

	"gorm.io/gorm"
)

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
			return ErrStockInsufficient
		}
		return err
	}
	return nil
}

func (s *InventoryService) Return(productID uint, quantity int) error {
	return s.repo.ReturnStock(productID, quantity)
}

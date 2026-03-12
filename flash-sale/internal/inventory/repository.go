package inventory

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrOptimisticLock = errors.New("optimistic lock conflict")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetByProductID(productID uint) (*Inventory, error) {
	var inv Inventory
	if err := r.db.Where("product_id = ?", productID).First(&inv).Error; err != nil {
		return nil, err
	}

	return &inv, nil
}

func (r *Repository) Upsert(inv *Inventory) error {
	return r.db.Save(inv).Error
}

func (r *Repository) DeductWithLock(productID uint, quantity int, version int) error {
	result := r.db.Model(&Inventory{}).
		Where("product_id = ? AND available >= ? AND version = ?", productID, quantity, version).
		Updates(map[string]interface{}{
			"available": gorm.Expr("available - ?", quantity),
			"locked":    gorm.Expr("locked + ?", quantity),
			"version":   gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOptimisticLock
	}
	return nil
}

func (r *Repository) ReturnStock(productID uint, quantity int) error {
	result := r.db.Model(&Inventory{}).
		Where("product_id = ? AND locked >= ?", productID, quantity).
		Updates(map[string]interface{}{
			"available": gorm.Expr("available + ?", quantity),
			"locked":    gorm.Expr("locked - ?", quantity),
			"version":   gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("insufficient locked stock to return")
	}
	return nil
}

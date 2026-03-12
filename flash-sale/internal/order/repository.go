package order

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(o *Order) error {
	return r.db.Create(o).Error
}

func (r *Repository) GetOrderByID(id uint) (*Order, error) {
	var o Order
	if err := r.db.First(&o, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *Repository) GetOrderByUserID(userID uint, page, pageSize int) ([]Order, int64, error) {
	var orders []Order
	var total int64

	r.db.Model(&Order{}).Where("user_id = ?", userID).Count(&total)

	offset := (page - 1) * pageSize
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&orders).Error

	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *Repository) UpdateStatus(orderID uint, status int16) error {
	return r.db.Model(&Order{}).Where("id = ?", orderID).Update("status", status).Error
}

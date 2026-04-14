package order

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Transaction wraps fn in a database transaction, so the service layer
// can coordinate cross-table atomicity without holding a *gorm.DB directly.
func (r *Repository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *Repository) Create(o *Order) error {
	return r.db.Create(o).Error
}

// CreateTx creates an order within the given transaction.
func (r *Repository) CreateTx(tx *gorm.DB, o *Order) error {
	return tx.Create(o).Error
}

func (r *Repository) GetOrderByID(id uint) (*Order, error) {
	var o Order
	if err := r.db.First(&o, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

// GetOrderByIDForUpdate retrieves an order with a row-level lock within the given transaction.
func (r *Repository) GetOrderByIDForUpdate(tx *gorm.DB, id uint) (*Order, error) {
	var o Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&o, "id = ?", id).Error; err != nil {
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

// UpdateStatusTx updates order status within the given transaction.
func (r *Repository) UpdateStatusTx(tx *gorm.DB, orderID uint, status int16) error {
	return tx.Model(&Order{}).Where("id = ?", orderID).Update("status", status).Error
}

func (r *Repository) ExistsByUserAndProduct(userID, productID uint) (bool, error) {
	var count int64
	err := r.db.Model(&Order{}).Where("user_id = ? AND product_id = ?", userID, productID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) GetOrderByOrderNo(orderNo string) (*Order, error) {
	var o Order
	if err := r.db.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *Repository) UpdateStatusByOrderNo(orderNo string, status int16) error {
	result := r.db.Model(&Order{}).Where("order_no = ?", orderNo).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateStatusByOrderNoTx(tx *gorm.DB, orderNo string, status int16) error {
	result := tx.Model(&Order{}).Where("order_no = ?", orderNo).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

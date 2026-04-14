package order

import "time"

// Order status constants
const (
	StatusPending         int16 = 0
	StatusAwaitingPayment int16 = 1
	StatusPaid            int16 = 2
	StatusCancelled       int16 = 3
)

type Order struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OrderNo    string    `gorm:"size:64;uniqueIndex;not null" json:"order_no"`
	UserID     uint      `gorm:"uniqueIndex:idx_user_product;not null" json:"user_id"`
	ProductID  uint      `gorm:"uniqueIndex:idx_user_product;not null" json:"product_id"`
	Quantity   int       `gorm:"not null" json:"quantity"`
	TotalPrice float64   `gorm:"type:decimal(10,2);not null" json:"total_price"`
	Status     int16     `gorm:"type:int;default:0" json:"status"` // 0=pending, 1=awaiting_payment, 2=paid, 3=cancelled
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type CreateOrderRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1"`
}

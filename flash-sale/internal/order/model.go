package order

import "time"

type Order struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OrderNo    string    `gorm:"size:32;uniqueIndex;not null" json:"order_no"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	ProductID  uint      `gorm:"index;not null" json:"product_id"`
	Quantity   int       `gorm:"not null" json:"quantity"`
	TotalPrice float64   `gorm:"type:decimal(10,2);not null" json:"total_price"`
	Status     int16     `gorm:"type:int;default:0" json:"status"` // 0=pending, 1=paid, 2=cancelled
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

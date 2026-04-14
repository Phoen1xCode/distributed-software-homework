package payment

import "time"

type Payment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PaymentID string    `gorm:"size:64;uniqueIndex;not null" json:"payment_id"`
	OrderNo   string    `gorm:"size:64;index;not null" json:"order_no"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	Amount    float64   `gorm:"type:decimal(10,2);not null" json:"amount"`
	Status    int16     `gorm:"type:int;default:0" json:"status"` // 0=pending, 1=success, 2=failed
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

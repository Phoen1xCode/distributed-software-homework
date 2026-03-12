package inventory

import "time"

type Inventory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"uniqueIndex;not null" json:"product_id"`
	Total     int       `gorm:"not null" json:"total"`
	Available int       `gorm:"not null" json:"available"`
	Locked    int       `gorm:"default:0" json:"locked"`
	Version   int       `gorm:"default:0" json:"version"` // optimistic lock
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type SetInventoryRequest struct {
	Total int `json:"total" binding:"required,gte=0"`
}

type DeductRequest struct {
	Quantity int `json:"quantity" binding:"required,gt=0"`
}


package product

import "time"

type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Price       float64   `gorm:"type:decimal(10,2);not null" json:"price"`
	Category    string    `gorm:"type:varchar(255)" json:"category"`
	ImageURL    string    `gorm:"type:varchar(255)" json:"image_url"`
	Status      int16     `gorm:"type:int;default:1" json:"status"` // 0=off, 1=on
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

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

type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required,gt=0"`
	Category    string  `json:"category"`
	ImageURL    string  `json:"image_url"`
}

type CreateProductResponse struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	ImageURL    string  `json:"image_url"`
	Status      *int16  `json:"status"`
}

type UpdateProductRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"omitempty,gt=0"`
	Category    string  `json:"category"`
	ImageURL    string  `json:"image_url"`
	Status      *int16  `json:"status"`
}

// ProductDetailResponse is used by GET /products/:id to return product with inventory.
// The inventory field is populated after the Inventory module is wired.
type ProductDetailResponse struct {
	Product   Product     `json:"product"`
	Inventory interface{} `json:"inventory,omitempty"`
}

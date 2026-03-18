package product

// ProductServicer defines the product service contract.
// Both ProductService (DB-backed) and CachedProductService (Redis-decorated)
// implement this interface.
type ProductServicer interface {
	CreateProduct(req *CreateProductRequest) (*Product, error)
	GetProductByID(id uint) (*Product, error)
	GetProductByName(name string) (*Product, error)
	ListProducts(page, pageSize int) ([]Product, int64, error)
	Update(id uint, req *UpdateProductRequest) (*Product, error)
	Delete(id uint) error
	GetPrice(id uint) (float64, error)
}

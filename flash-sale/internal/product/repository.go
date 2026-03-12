package product

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(p *Product) error {
	return r.db.Create(p).Error
}

func (r *Repository) FindProductByID(id uint) (*Product, error) {
	var p Product
	if err := r.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) FindProductByName(name string) (*Product, error) {
	var p Product
	if err := r.db.First(&p, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) FindAllProducts(page, pageSize int) ([]Product, int64, error) {
	var products []Product
	var total int64

	r.db.Model(&Product{}).Where("status = ?", 1).Count(&total)

	offset := (page - 1) * pageSize
	err := r.db.Where("status = ?", 1).
		Order("created_at desc").
		Offset(offset).Limit(pageSize).
		Find(&products).Error

	return products, total, err
}

func (r *Repository) Update(p *Product) error {
	return r.db.Save(p).Error
}

func (r *Repository) Delete(id uint) error {
	return r.db.Delete(&Product{}, "id = ?", id).Error
}

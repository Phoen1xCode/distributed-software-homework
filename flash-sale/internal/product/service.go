package product

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrProductNotFound = errors.New("product not found")
)

type ProductService struct {
	repo *Repository
}

func NewProductService(repo *Repository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) CreateProduct(req *CreateProductRequest) (*Product, error) {
	p := &Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImageURL:    req.ImageURL,
		Status:      1,
	}
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProductService) GetProductByID(id uint) (*Product, error) {
	p, err := s.repo.FindProductByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return p, nil
}

func (s *ProductService) GetProductByName(name string) (*Product, error) {
	p, err := s.repo.FindProductByName(name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return p, nil
}

func (s *ProductService) ListProducts(page, pageSize int) ([]Product, int64, error) {
	return s.repo.FindAllProducts(page, pageSize)
}

func (s *ProductService) Update(id uint, req *UpdateProductRequest) (*Product, error) {
	p, err := s.repo.FindProductByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.Price > 0 {
		p.Price = req.Price
	}
	if req.Category != "" {
		p.Category = req.Category
	}
	if req.ImageURL != "" {
		p.ImageURL = req.ImageURL
	}
	if req.Status != nil {
		p.Status = *req.Status
	}

	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProductService) Delete(id uint) error {
	_, err := s.repo.FindProductByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

func (s *ProductService) GetPrice(id uint) (float64, error) {
	p, err := s.repo.FindProductByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrProductNotFound
		}
		return 0, err
	}
	return p.Price, nil
}

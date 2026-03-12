package product

import (
	"errors"
	"flash-sale/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *ProductService
}

func NewHandler(service *ProductService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	p, err := h.service.CreateProduct(&req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, p)
}

func (h *Handler) GetProductByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	p, err := h.service.GetProductByID(uint(id))
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, p)
}

func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	products, total, err := h.service.ListProducts(page, pageSize)
	if err != nil {
		response.InternalServerError(c, "failed to list products")
		return
	}
	response.SuccessPaginated(c, products, total, int64(page), int64(pageSize))
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	p, err := h.service.Update(uint(id), &req)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to update product")
		return
	}
	response.Success(c, p)
}

func (h *Handler) DeleteProduct(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		if errors.Is(err, ErrProductNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to delete product")
		return
	}
	response.Success(c, nil)
}

func (h *Handler) RegisterRoutes(publicGroup, adminGroup *gin.RouterGroup) {
	publicGroup.GET("/products", h.List)
	publicGroup.GET("/products/:id", h.GetProductByID)

	adminGroup.POST("/products", h.CreateProduct)
	adminGroup.PUT("/products/:id", h.UpdateProduct)
	adminGroup.DELETE("/products/:id", h.DeleteProduct)
}

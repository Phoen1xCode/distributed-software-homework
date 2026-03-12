package inventory

import (
	"errors"
	"flash-sale/pkg/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *InventoryService
}

func NewHandler(service *InventoryService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Get(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}

	inv, err := h.service.GetByProductID(uint(productID))
	if err != nil {
		if errors.Is(err, ErrInventoryNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to get inventory")
		return
	}
	response.Success(c, inv)
}

func (h *Handler) Set(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}

	var req SetInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	inv, err := h.service.SetInventory(uint(productID), &req)
	if err != nil {
		response.InternalServerError(c, "failed to set inventory")
		return
	}
	response.Success(c, inv)
}

func (h *Handler) Deduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}

	var req DeductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.service.Deduct(uint(productID), req.Quantity); err != nil {
		if errors.Is(err, ErrInventoryNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrStockInsufficient) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.InternalServerError(c, "failed to deduct inventory")
		return
	}
	response.Success(c, nil)
}

func (h *Handler) RegisterRoutes(publicGroup, authGroup, adminGroup *gin.RouterGroup) {
	publicGroup.GET("/inventory/:product_id", h.Get)
	adminGroup.PUT("/inventory/:product_id", h.Set)
	adminGroup.POST("/inventory/:product_id/deduct", h.Deduct)
}

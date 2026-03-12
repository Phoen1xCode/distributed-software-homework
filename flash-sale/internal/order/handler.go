package order

import (
	"errors"
	"flash-sale/pkg/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *OrderService
}

func NewHandler(service *OrderService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateOrder(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "Unauthorized")
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	o, err := h.service.CreateOrder(userID.(uint), &req)

	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			response.NotFound(c, err.Error())
			return
		}

		if errors.Is(err, ErrProductOffSale) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}

		if errors.Is(err, ErrStockInsufficient) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}

		response.InternalServerError(c, "failed to create order")
	}
	response.Success(c, o)
}

func (h *Handler) ListOrders(c *gin.Context) {
	userID, _ := c.Get("user_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	orders, total, err := h.service.ListOrderByUser(userID.(uint), page, pageSize)
	if err != nil {
		response.InternalServerError(c, "failed to list orders")
		return
	}
	response.SuccessPaginated(c, orders, total, int64(page), int64(pageSize))
}

func (h *Handler) GetOrderByID(c *gin.Context) {
	userID, _ := c.Get("user_id")

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid order id")
		return
	}

	o, err := h.service.GetOrderByID(userID.(uint), uint(id))
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotOrderOwner) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to get order")
		return
	}
	response.Success(c, o)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid order id")
		return
	}

	o, err := h.service.CancelOrder(userID.(uint), uint(id))
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotOrderOwner) {
			response.Forbidden(c, err.Error())
			return
		}
		if errors.Is(err, ErrOrderNotPending) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.InternalServerError(c, "failed to cancel order")
		return
	}
	response.Success(c, o)
}

func (h *Handler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.POST("/orders", h.CreateOrder)
	authGroup.GET("/orders", h.ListOrders)
	authGroup.GET("/orders/:id", h.GetOrderByID)
	authGroup.PUT("/orders/:id/cancel", h.CancelOrder)
}

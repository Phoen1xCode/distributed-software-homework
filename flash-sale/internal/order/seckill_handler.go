package order

import (
	"errors"
	"flash-sale/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SeckillHandler struct {
	service *SeckillService
}

func NewSeckillHandler(service *SeckillService) *SeckillHandler {
	return &SeckillHandler{service: service}
}

func (h *SeckillHandler) Seckill(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	orderNo, err := h.service.Seckill(userID.(uint), &req)
	if err != nil {
		if errors.Is(err, ErrDuplicateOrder) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrProductNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrProductOffSale) || errors.Is(err, ErrStockInsufficient) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.InternalServerError(c, "failed to process seckill")
		return
	}

	response.Success(c, gin.H{
		"order_no": orderNo,
		"message":  "order is being processed",
	})
}

func (h *SeckillHandler) GetResult(c *gin.Context) {
	orderNo := c.Query("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	status, err := h.service.GetSeckillResult(orderNo)
	if err != nil {
		response.InternalServerError(c, "failed to get result")
		return
	}

	response.Success(c, gin.H{
		"order_no": orderNo,
		"status":   status,
	})
}

func (h *SeckillHandler) RegisterRoutes(authGroup *gin.RouterGroup) {
	authGroup.POST("/seckill", h.Seckill)
	authGroup.GET("/seckill/result", h.GetResult)
}

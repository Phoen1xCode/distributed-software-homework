package user

import (
	"errors"
	"flash-sale/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *UserService
}

func NewHandler(service *UserService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.service.Register(&req)
	if err != nil {
		if errors.Is(err, ErrUserExists) || errors.Is(err, ErrEmailExists) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.InternalServerError(c, "registration failed")
		return
	}
	response.Success(c, user)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.service.Login(&req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Unauthorized(c, err.Error())
			return
		}
		response.InternalServerError(c, "login failed")
		return
	}

	response.Success(c, resp)
}

func (h *Handler) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	user, err := h.service.GetProfile(userID.(uint))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to get profile")
		return
	}

	response.Success(c, user)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.service.UpdateProfile(userID.(uint), &req)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrEmailExists) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.InternalServerError(c, "failed to update profile")
		return
	}

	response.Success(c, user)
}

func (h *Handler) RegisterRoutes(publicGroup *gin.RouterGroup, authGroup *gin.RouterGroup) {
	publicGroup.POST("/auth/register", h.Register)
	publicGroup.POST("/auth/login", h.Login)

	authGroup.GET("/user/profile", h.GetProfile)
	authGroup.PUT("/user/profile", h.UpdateProfile)
}

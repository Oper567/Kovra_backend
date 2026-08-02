package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kovra-dev/kovra/backend/vtu-gaming-service/internal/domain"
	"github.com/kovra-dev/kovra/backend/vtu-gaming-service/internal/usecase"
	"github.com/shopspring/decimal"
)

type AdminVTUHandler struct {
	uc *usecase.VTUUsecase
}

func NewAdminVTUHandler(uc *usecase.VTUUsecase) *AdminVTUHandler {
	return &AdminVTUHandler{uc: uc}
}

func (h *AdminVTUHandler) RegisterRoutes(r *gin.RouterGroup) {
	admin := r.Group("/vtu/admin")
	// In production, add auth middleware requiring admin roles here
	{
		admin.POST("/products", h.CreateProduct)
		admin.PUT("/products/:id", h.UpdateProduct)
		admin.DELETE("/products/:id", h.DeleteProduct)
	}
}

type ProductRequest struct {
	ProviderID   string          `json:"provider_id" binding:"required"`
	Category     string          `json:"category" binding:"required"`
	Name         string          `json:"name" binding:"required"`
	Description  string          `json:"description"`
	Amount       decimal.Decimal `json:"amount" binding:"required"`
	Currency     string          `json:"currency"`
	ProviderCode string          `json:"provider_code" binding:"required"`
	IsActive     bool            `json:"is_active"`
}

func (h *AdminVTUHandler) CreateProduct(c *gin.Context) {
	var req ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	desc := req.Description

	product := &domain.Product{
		ProviderID:   req.ProviderID,
		Category:     domain.VTUCategory(req.Category),
		Name:         req.Name,
		Description:  &desc,
		Amount:       req.Amount,
		Currency:     req.Currency,
		ProviderCode: req.ProviderCode,
		IsActive:     req.IsActive,
	}

	if err := h.uc.CreateProduct(c.Request.Context(), product); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{Success: true, Data: product})
}

func (h *AdminVTUHandler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "missing id"})
		return
	}

	var req ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	desc := req.Description

	product := &domain.Product{
		ID:           id,
		ProviderID:   req.ProviderID,
		Category:     domain.VTUCategory(req.Category),
		Name:         req.Name,
		Description:  &desc,
		Amount:       req.Amount,
		Currency:     req.Currency,
		ProviderCode: req.ProviderCode,
		IsActive:     req.IsActive,
	}

	if err := h.uc.UpdateProduct(c.Request.Context(), product); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: product})
}

func (h *AdminVTUHandler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "missing id"})
		return
	}

	if err := h.uc.DeleteProduct(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: "Product deactivated"})
}

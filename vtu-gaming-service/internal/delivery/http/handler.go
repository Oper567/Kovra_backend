package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/domain"
	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/usecase"
)

type VTUHandler struct {
	uc *usecase.VTUUsecase
}

func NewVTUHandler(uc *usecase.VTUUsecase) *VTUHandler {
	return &VTUHandler{uc: uc}
}

func (h *VTUHandler) RegisterRoutes(r *gin.RouterGroup) {
	vtu := r.Group("/vtu")
	{
		vtu.GET("/products", h.ListProducts)
		vtu.POST("/purchase", h.Purchase)
		vtu.GET("/orders", h.ListOrders)
		vtu.GET("/validate-meter", h.ValidateMeter)
	}
}

type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (h *VTUHandler) ListProducts(c *gin.Context) {
	category := c.Query("category")
	products, err := h.uc.ListProducts(c.Request.Context(), category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: products})
}

type PurchaseRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Recipient string `json:"recipient" binding:"required"`
}

func (h *VTUHandler) Purchase(c *gin.Context) {
	var req PurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Error: "unauthorized"})
		return
	}

	resp, err := h.uc.Purchase(c.Request.Context(), usecase.PurchaseRequest{
		UserID:    userID,
		ProductID: req.ProductID,
		Recipient: req.Recipient,
	})

	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: resp})
}

func (h *VTUHandler) ListOrders(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Error: "unauthorized"})
		return
	}

	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	orders, nextCursor, err := h.uc.ListOrders(c.Request.Context(), userID, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"orders":      orders,
			"next_cursor": nextCursor,
		},
	})
}

func (h *VTUHandler) handleError(c *gin.Context, err error) {
	if errors.Is(err, domain.ErrProductNotFound) {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
	} else if errors.Is(err, domain.ErrProductInactive) {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
	} else {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
	}
}

func (h *VTUHandler) ValidateMeter(c *gin.Context) {
	meterNumber := c.Query("meternumber")
	discoName := c.Query("disconame")
	meterType := c.Query("mtype")

	if meterNumber == "" || discoName == "" || meterType == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "missing required parameters: meternumber, disconame, mtype"})
		return
	}

	res, err := h.uc.ValidateMeter(c.Request.Context(), meterNumber, discoName, meterType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: res})
}

package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/edtech-service/internal/usecase"
)

type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

type EdtechHandler struct {
	uc *usecase.EdtechUsecase
}

func NewEdtechHandler(uc *usecase.EdtechUsecase) *EdtechHandler {
	return &EdtechHandler{uc: uc}
}

func (h *EdtechHandler) RegisterRoutes(r *gin.RouterGroup) {
	edtech := r.Group("/edtech")
	{
		edtech.POST("/execute-code", h.ExecuteCode)
		edtech.POST("/certificate/purchase", h.PurchaseCertificate)
	}
}

func (h *EdtechHandler) ExecuteCode(c *gin.Context) {
	var req usecase.ExecuteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	res, err := h.uc.ExecuteCode(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: res})
}

func (h *EdtechHandler) PurchaseCertificate(c *gin.Context) {
	var req usecase.PurchaseCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = c.GetHeader("X-User-ID")
	}
	if userID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Error: "Unauthorized"})
		return
	}
	req.UserID = userID

	res, err := h.uc.PurchaseCertificate(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: res})
}

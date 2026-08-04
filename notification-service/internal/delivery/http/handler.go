package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/notification-service/internal/usecase"
)

type Handler struct {
	uc *usecase.NotificationUsecase
}

func NewHandler(uc *usecase.NotificationUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Private network routing only via API Gateway
	api := r.Group("/api/v1/notifications")
	{
		api.POST("/devices/register", h.registerDevice)
		api.POST("/admin/push", h.adminPush) // In prod, protect this with an admin API key middleware
	}
}

func (h *Handler) registerDevice(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		FCMToken string `json:"fcm_token" binding:"required"`
		Platform string `json:"platform" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.uc.RegisterDevice(c.Request.Context(), req.UserID, req.FCMToken, req.Platform); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "device registered"})
}

func (h *Handler) adminPush(c *gin.Context) {
	var req struct {
		Title string `json:"title" binding:"required"`
		Body  string `json:"body" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.uc.SendAdminBroadcast(c.Request.Context(), req.Title, req.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send broadcast"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

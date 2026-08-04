package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/engagement-service/internal/usecase"
)

type EngagementHandler struct {
	uc *usecase.EngagementUsecase
}

func NewEngagementHandler(uc *usecase.EngagementUsecase) *EngagementHandler {
	return &EngagementHandler{uc: uc}
}

func (h *EngagementHandler) RegisterRoutes(r *gin.RouterGroup) {
	user := r.Group("/user")
	{
		user.GET("/profile", h.GetProfile)
	}

	rewards := r.Group("/rewards")
	{
		rewards.GET("/dashboard", h.GetRewardsDashboard)
		rewards.POST("/claim", h.ClaimReward)
	}
}

func (h *EngagementHandler) GetProfile(c *gin.Context) {
	// Stub implementation to stabilize routing and provide mock data for the frontend
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"name":        "Joshua Doe",
			"handle":      "joshua_d",
			"avatar_url":  "assets/images/avatar_placeholder.png",
			"kyc_status":  true,
			"loyalty_tier": "Gold Tier",
			"xp_points":   1250,
		},
	})
}

func (h *EngagementHandler) GetRewardsDashboard(c *gin.Context) {
	// Stub implementation for frontend compatibility
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_xp":      1250,
			"current_tier":  "Silver Member",
			"xp_to_next_tier": 750,
		},
	})
}

func (h *EngagementHandler) ClaimReward(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

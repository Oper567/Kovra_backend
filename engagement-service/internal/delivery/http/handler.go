package http

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/engagement-service/internal/usecase"
)

type EngagementHandler struct {
	uc *usecase.EngagementUsecase
	db *sql.DB
}

func NewEngagementHandler(uc *usecase.EngagementUsecase, db *sql.DB) *EngagementHandler {
	return &EngagementHandler{uc: uc, db: db}
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
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"name":         "Joshua Doe",
			"handle":       "joshua_d",
			"avatar_url":   "assets/images/avatar_placeholder.png",
			"kyc_status":   true,
			"loyalty_tier": "Gold Tier",
			"xp_points":    1250,
		},
	})
}

func (h *EngagementHandler) GetRewardsDashboard(c *gin.Context) {
	userID := c.GetString("user_id") // set by auth middleware
	if userID == "" {
		// fallback to a mock ID for local testing if auth is disabled
		userID = "00000000-0000-0000-0000-000000000000"
	}

	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"total_xp":        1250,
				"current_tier":    "Silver Member",
				"xp_to_next_tier": 750,
			},
		})
		return
	}

	var totalXp int
	var tier string
	err := h.db.QueryRowContext(c.Request.Context(), `
		SELECT total_xp, tier FROM engagement_user_xp WHERE user_id = $1
	`, userID).Scan(&totalXp, &tier)
	
	if err != nil {
		if err == sql.ErrNoRows {
			totalXp = 0
			tier = "Bronze Member"
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_xp":        totalXp,
			"current_tier":    tier,
			"xp_to_next_tier": 500, // Dummy calculation for now
		},
	})
}

func (h *EngagementHandler) ClaimReward(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

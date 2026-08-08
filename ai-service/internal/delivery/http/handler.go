package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AIUsecase defines the interface for the AI logic tier.
type AIUsecase interface {
	GenerateLuciInsight(ctx context.Context, userID string, viewContext string) (string, error)
	EvaluateMerchant(ctx context.Context, storeName, description string) (string, error)
	EvaluateTutor(ctx context.Context, displayName, bio string) (string, error)
}

// AIHandler handles HTTP requests for AI endpoints.
type AIHandler struct {
	usecase AIUsecase
}

// NewAIHandler initializes the handler and registers routes on the Gin engine.
func NewAIHandler(r *gin.Engine, usecase AIUsecase) {
	handler := &AIHandler{
		usecase: usecase,
	}

	// Route Grouping with JWT Middleware
	// Assuming authMiddleware is implemented to extract and set "userID"
	aiGroup := r.Group("/api/v1/ai")
	aiGroup.Use(authMiddleware())

	aiGroup.POST("/insight", handler.GetInsight)
	aiGroup.POST("/review/merchant", handler.ReviewMerchant)
	aiGroup.POST("/review/tutor", handler.ReviewTutor)
}

// InsightRequest maps the incoming JSON payload.
type InsightRequest struct {
	Context string `json:"context" binding:"required"`
}

// InsightResponse maps the outgoing JSON payload.
type InsightResponse struct {
	Message string `json:"message"`
}

type ReviewMerchantRequest struct {
	StoreName   string `json:"store_name" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type ReviewTutorRequest struct {
	DisplayName string `json:"display_name" binding:"required"`
	Bio         string `json:"bio" binding:"required"`
}

// GetInsight handles POST /api/v1/ai/insight
func (h *AIHandler) GetInsight(c *gin.Context) {
	var req InsightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format. 'context' is required."})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized access"})
		return
	}

	uidStr, ok := userID.(string)
	if !ok || uidStr == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: Invalid user identity"})
		return
	}

	msg, _ := h.usecase.GenerateLuciInsight(c.Request.Context(), uidStr, req.Context)

	c.JSON(http.StatusOK, InsightResponse{
		Message: msg,
	})
}

func (h *AIHandler) ReviewMerchant(c *gin.Context) {
	var req ReviewMerchantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format"})
		return
	}

	status, err := h.usecase.EvaluateMerchant(c.Request.Context(), req.StoreName, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate merchant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *AIHandler) ReviewTutor(c *gin.Context) {
	var req ReviewTutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format"})
		return
	}

	status, err := h.usecase.EvaluateTutor(c.Request.Context(), req.DisplayName, req.Bio)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate tutor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set("userID", userID)
		}
		c.Next()
	}
}

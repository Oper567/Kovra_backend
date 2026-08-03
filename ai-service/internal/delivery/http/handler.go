package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AIUsecase defines the interface for the AI logic tier.
type AIUsecase interface {
	GenerateKoviInsight(ctx context.Context, userID string, viewContext string) (string, error)
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
}

// InsightRequest maps the incoming JSON payload.
type InsightRequest struct {
	Context string `json:"context" binding:"required"`
}

// InsightResponse maps the outgoing JSON payload.
type InsightResponse struct {
	Message string `json:"message"`
}

// GetInsight handles POST /api/v1/ai/insight
func (h *AIHandler) GetInsight(c *gin.Context) {
	var req InsightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format. 'context' is required."})
		return
	}

	// Extract userID from the Gin context (populated by authMiddleware)
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

	// Call the Usecase
	// We ignore the error here because GenerateKoviInsight is guaranteed to return
	// a safe fallback string instead of failing, ensuring the frontend UI mascot doesn't break.
	msg, _ := h.usecase.GenerateKoviInsight(c.Request.Context(), uidStr, req.Context)

	// Return the Mascot Message
	c.JSON(http.StatusOK, InsightResponse{
		Message: msg,
	})
}

// authMiddleware is a placeholder for the actual JWT middleware implementation
// which extracts the user ID from the Bearer token and sets it in the Gin context.
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mock implementation for structural correctness
		// c.Set("userID", "extracted_user_id_from_jwt")
		c.Next()
	}
}

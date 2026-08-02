package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kovra-dev/kovra/backend/ai-service/internal/usecase"
)

type aiHandler struct {
	aiUsecase *usecase.AIUsecase
}

func RegisterRoutes(r *gin.Engine, uc *usecase.AIUsecase) {
	h := &aiHandler{aiUsecase: uc}
	
	api := r.Group("/api/v1/ai")
	{
		api.POST("/chat", h.chat)
		api.GET("/chat/sessions", h.listSessions)
		api.GET("/chat/sessions/:id/messages", h.getHistory)
		
		api.GET("/insights", h.getInsights)
		api.GET("/recommendations", h.getRecommendations)
		api.POST("/recommendations/:id/dismiss", h.dismissRecommendation)
	}
}

func (h *aiHandler) chat(c *gin.Context) {
	// In production, get UserID from JWT context
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user id required"})
		return
	}

	var req usecase.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	res, err := h.aiUsecase.Chat(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": res})
}

func (h *aiHandler) listSessions(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	sessions, err := h.aiUsecase.ListChatSessions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sessions})
}

func (h *aiHandler) getHistory(c *gin.Context) {
	sessionID := c.Param("id")
	messages, err := h.aiUsecase.GetChatHistory(c.Request.Context(), sessionID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": messages})
}

func (h *aiHandler) getInsights(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	insights, err := h.aiUsecase.GetInsights(c.Request.Context(), userID, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": insights})
}

func (h *aiHandler) getRecommendations(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	recType := c.Query("type") // e.g. "product", "course"
	if recType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type query param required"})
		return
	}
	
	recs, err := h.aiUsecase.GetRecommendations(c.Request.Context(), userID, recType, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": recs})
}

func (h *aiHandler) dismissRecommendation(c *gin.Context) {
	recID := c.Param("id")
	if err := h.aiUsecase.DismissRecommendation(c.Request.Context(), recID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

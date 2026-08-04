package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/edtech-service/internal/usecase"
)

type QuizHandler struct {
	uc *usecase.QuizUsecase
}

func NewQuizHandler(uc *usecase.QuizUsecase) *QuizHandler {
	return &QuizHandler{uc: uc}
}

func (h *QuizHandler) RegisterRoutes(r *gin.RouterGroup) {
	edtech := r.Group("/edtech")
	{
		edtech.POST("/quizzes/create", h.CreateQuiz)
	}
}

func (h *QuizHandler) CreateQuiz(c *gin.Context) {
	var req usecase.CreateQuizRequest
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
	req.TutorID = userID

	res, err := h.uc.CreateQuiz(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{Success: true, Data: res})
}

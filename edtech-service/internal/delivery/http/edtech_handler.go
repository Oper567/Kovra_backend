package http

import (
	"database/sql"
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
	db *sql.DB
}

func NewEdtechHandler(uc *usecase.EdtechUsecase, db *sql.DB) *EdtechHandler {
	return &EdtechHandler{uc: uc, db: db}
}

func (h *EdtechHandler) RegisterRoutes(r *gin.RouterGroup) {
	edtech := r.Group("/edtech")
	{
		edtech.POST("/execute-code", h.ExecuteCode)
		edtech.POST("/certificate/purchase", h.PurchaseCertificate)
		edtech.GET("/tutor/dashboard", h.GetDashboardData)
		edtech.GET("/assessment/active", h.GetActiveAssessment)
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

func (h *EdtechHandler) GetDashboardData(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "00000000-0000-0000-0000-000000000000" // mock fallback
	}

	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"metrics": gin.H{"total_students": 150, "active_courses": 3, "quizzes_taken": 12, "total_earnings": 150000.0}, "courses": []any{}}})
		return
	}

	// Fetch tutor metrics from DB
	var tutorID string
	err := h.db.QueryRowContext(c.Request.Context(), "SELECT id FROM edtech_tutors WHERE user_id = $1", userID).Scan(&tutorID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"metrics": gin.H{"total_students": 0, "active_courses": 0, "quizzes_taken": 0, "total_earnings": 0}, "courses": []any{}}})
			return
		}
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Database error"})
		return
	}

	var activeCourses int
	var totalEarnings float64
	var totalStudents int

	// Dummy logic to simulate live data aggregation (can be replaced with complex joins)
	h.db.QueryRowContext(c.Request.Context(), "SELECT COUNT(*), SUM(price * enrollment_count), SUM(enrollment_count) FROM edtech_courses WHERE tutor_id = $1", tutorID).Scan(&activeCourses, &totalEarnings, &totalStudents)

	// Fetch courses
	rows, err := h.db.QueryContext(c.Request.Context(), "SELECT id, title, enrollment_count FROM edtech_courses WHERE tutor_id = $1", tutorID)
	
	var courses []map[string]any
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title string
			var enrollment int
			if err := rows.Scan(&id, &title, &enrollment); err == nil {
				courses = append(courses, map[string]any{
					"id": id,
					"title": title,
					"student_count": enrollment,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"metrics": gin.H{
				"total_students": totalStudents,
				"active_courses": activeCourses,
				"quizzes_taken": 0, // Placeholder
				"total_earnings": totalEarnings,
			},
			"courses": courses,
		},
	})
}

func (h *EdtechHandler) GetActiveAssessment(c *gin.Context) {
	// Stub implementation for now, returning empty state to allow UI to render without errors
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"title": "No Active Assessment",
			"security_settings": gin.H{
				"camera": false,
				"lockdown": false,
				"ai_feedback": false,
			},
			"submissions": []any{},
		},
	})
}

package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
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
		edtech.POST("/tutor", h.CreateTutor)
		edtech.POST("/execute-code", h.ExecuteCode)
		edtech.POST("/certificate/purchase", h.PurchaseCertificate)
		edtech.GET("/tutor/dashboard", h.GetDashboardData)
		edtech.GET("/assessment/active", h.GetActiveAssessment)
		edtech.GET("/tutors/top", h.GetTopTutors)
		edtech.GET("/courses/my-learning", h.GetMyLearning)
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
	var tutorID, status string
	err := h.db.QueryRowContext(c.Request.Context(), "SELECT id, status FROM edtech_tutors WHERE user_id = $1", userID).Scan(&tutorID, &status)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "none", "metrics": gin.H{"total_students": 0, "active_courses": 0, "quizzes_taken": 0, "total_earnings": 0}, "courses": []any{}}})
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
			"status": status,
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

type CreateTutorRequest struct {
	DisplayName     string   `json:"display_name" binding:"required"`
	Bio             string   `json:"bio"`
	AvatarURL       string   `json:"avatar_url"`
	Specializations []string `json:"specializations"`
}

func (h *EdtechHandler) CreateTutor(c *gin.Context) {
	var req CreateTutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid payload"})
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

	// Insert tutor with pending status
	var tutorID string
	query := `INSERT INTO edtech_tutors (user_id, display_name, bio, avatar_url, status) 
			  VALUES ($1, $2, $3, $4, 'pending') RETURNING id`
	
	err := h.db.QueryRowContext(c.Request.Context(), query, userID, req.DisplayName, req.Bio, req.AvatarURL).Scan(&tutorID)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{Success: false, Error: "User is already a tutor or db error"})
		return
	}

	// Async AI review
	go func(id, name, bio string) {
		// Simulate network call to AI service
		payload := map[string]string{"display_name": name, "bio": bio}
		jsonPayload, _ := json.Marshal(payload)
		resp, err := http.Post("http://127.0.0.1:8085/api/v1/ai/review/tutor", "application/json", bytes.NewBuffer(jsonPayload))
		if err == nil && resp.StatusCode == http.StatusOK {
			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)
			if result["status"] == "approved" {
				h.db.Exec("UPDATE edtech_tutors SET status = 'approved' WHERE id = $1", id)
			} else {
				h.db.Exec("UPDATE edtech_tutors SET status = 'rejected' WHERE id = $1", id)
			}
		}
	}(tutorID, req.DisplayName, req.Bio)

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{"id": tutorID, "status": "pending"}})
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

func (h *EdtechHandler) GetTopTutors(c *gin.Context) {
	// Stub implementation returning top tutors
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": []map[string]any{
			{"name": "Dr. Sarah", "subject": "Data Science", "rating": 4.9},
			{"name": "Prof. John", "subject": "Engineering", "rating": 4.8},
			{"name": "Alice M.", "subject": "UI/UX Design", "rating": 5.0},
			{"name": "Mike T.", "subject": "Golang", "rating": 4.7},
			{"name": "Emma W.", "subject": "Marketing", "rating": 4.9},
		},
	})
}

func (h *EdtechHandler) GetMyLearning(c *gin.Context) {
	// Stub implementation returning a user's enrolled courses
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": []map[string]any{
			{
				"id": "course-123",
				"title": "Advanced Flutter & Go",
				"module": "Module 4: Microservices",
				"progress": 0.65,
				"is_completed": false,
			},
		},
	})
}

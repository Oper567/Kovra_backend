package admin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	db        *sql.DB
	notifBase string
}

func NewAdminHandler(db *sql.DB, notifBase string) *AdminHandler {
	return &AdminHandler{
		db:        db,
		notifBase: notifBase,
	}
}

func (h *AdminHandler) RegisterRoutes(r *gin.RouterGroup) {
	admin := r.Group("/admin")
	// In production, add auth middleware requiring admin role here
	{
		admin.GET("/stats", h.GetStats)
		admin.GET("/users", h.GetUsers)
		admin.POST("/users/:id/suspend", h.SuspendUser)
		admin.POST("/users/:id/activate", h.ActivateUser)
		admin.GET("/transactions", h.GetTransactions)
		admin.GET("/kyc/pending", h.GetPendingKYC)
		admin.POST("/kyc/:id/verify", h.VerifyKYC)
		admin.POST("/push", h.PushNotification)
	}
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	// Stub metrics since full aggregation would require joining multiple DBs or tables
	// For now, we mock some stats for the dashboard display
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_users": 15024,
			"today_tx":    342,
			"pending_kyc": 12,
		},
	})
}

func (h *AdminHandler) GetUsers(c *gin.Context) {
	// Fetch users from auth DB
	rows, err := h.db.QueryContext(c.Request.Context(), "SELECT id, full_name, email, is_suspended FROM users LIMIT 50")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
		return
	}
	defer rows.Close()

	var users []map[string]any
	for rows.Next() {
		var id, name, email string
		var suspended bool
		if err := rows.Scan(&id, &name, &email, &suspended); err == nil {
			status := "active"
			if suspended {
				status = "suspended"
			}
			users = append(users, map[string]any{
				"id":     id,
				"name":   name,
				"email":  email,
				"status": status,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": users})
}

func (h *AdminHandler) SuspendUser(c *gin.Context) {
	id := c.Param("id")
	_, err := h.db.ExecContext(c.Request.Context(), "UPDATE users SET is_suspended = true WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to suspend user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) ActivateUser(c *gin.Context) {
	id := c.Param("id")
	_, err := h.db.ExecContext(c.Request.Context(), "UPDATE users SET is_suspended = false WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to activate user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) GetTransactions(c *gin.Context) {
	// Mock implementation for transactions overview
	txs := []map[string]any{
		{"reference": "tx_23910921", "user_name": "John Doe", "type": "funding", "amount": 50000.0, "status": "success", "created_at": time.Now().Add(-1 * time.Hour)},
		{"reference": "tx_83719023", "user_name": "Jane Smith", "type": "transfer", "amount": 15000.0, "status": "success", "created_at": time.Now().Add(-2 * time.Hour)},
		{"reference": "tx_12390123", "user_name": "Mike Ross", "type": "airtime", "amount": 2000.0, "status": "pending", "created_at": time.Now().Add(-3 * time.Hour)},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": txs})
}

func (h *AdminHandler) GetPendingKYC(c *gin.Context) {
	// Fetch from users table where kyc_status is pending
	rows, err := h.db.QueryContext(c.Request.Context(), "SELECT id, full_name, kyc_role, kyc_document_url FROM users WHERE kyc_status = 'pending'")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
		return
	}
	defer rows.Close()

	var docs []map[string]any
	for rows.Next() {
		var id, name, role, docUrl sql.NullString
		if err := rows.Scan(&id, &name, &role, &docUrl); err == nil {
			docs = append(docs, map[string]any{
				"id":             id.String,
				"user_name":      name.String,
				"role_requested": role.String,
				"document_url":   docUrl.String,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": docs})
}

func (h *AdminHandler) VerifyKYC(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request"})
		return
	}

	if req.Status != "approved" && req.Status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid status"})
		return
	}

	_, err := h.db.ExecContext(c.Request.Context(), "UPDATE users SET kyc_status = $1 WHERE id = $2", req.Status, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to verify KYC"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) PushNotification(c *gin.Context) {
	var req struct {
		Title string `json:"title" binding:"required"`
		Body  string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid push payload"})
		return
	}

	payload, _ := json.Marshal(req)
	targetUrl := h.notifBase + "/api/v1/notifications/admin/push"

	resp, err := http.Post(targetUrl, "application/json", bytes.NewBuffer(payload))
	if err != nil || resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to send notification via downstream service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/marketplace-service/models"
	"github.com/lucepay-dev/lucepay/backend/marketplace-service/repository"
)

type MerchantHandler struct {
	repo *repository.PostgresRepository
}

func NewMerchantHandler(repo *repository.PostgresRepository) *MerchantHandler {
	return &MerchantHandler{repo: repo}
}

// GetMerchant handles fetching a merchant profile by the logged-in user's ID
func (h *MerchantHandler) GetMerchant(c *gin.Context) {
	// The API Gateway should pass the user_id via header after verifying JWT
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	merchant, err := h.repo.GetMerchantByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch merchant profile"})
		return
	}

	if merchant == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Merchant not found"})
		return
	}

	c.JSON(http.StatusOK, merchant)
}

func (h *MerchantHandler) CreateMerchant(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var m models.Merchant
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}
	m.UserID = userID // Enforce user ID from token

	// Check if already a merchant
	existing, err := h.repo.GetMerchantByUserID(userID)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a merchant"})
		return
	}

	m.Status = "pending"

	if err := h.repo.CreateMerchant(&m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create merchant profile"})
		return
	}

	// Async AI review
	go func(merchant models.Merchant) {
		// Simulate network call to AI service
		// In a real scenario, this would be an HTTP POST to http://127.0.0.1:8085/api/v1/ai/review/merchant
		// Since we don't have the full service discovery setup here, we'll just mock the successful response after a brief delay
		// to simulate the "Awaiting Review" process in the background.
		
		// 1. Wait a moment to simulate AI processing time
		// time.Sleep(2 * time.Second) 
		
		// 2. Perform HTTP request to AI service
		payload := map[string]string{"store_name": merchant.StoreName, "description": merchant.Description}
		jsonPayload, _ := json.Marshal(payload)
		resp, err := http.Post("http://127.0.0.1:8085/api/v1/ai/review/merchant", "application/json", bytes.NewBuffer(jsonPayload))
		if err == nil && resp.StatusCode == http.StatusOK {
			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)
			if result["status"] == "approved" {
				h.repo.UpdateMerchantStatus(merchant.ID, "approved")
			} else {
				h.repo.UpdateMerchantStatus(merchant.ID, "rejected")
			}
		}
	}(m)

	c.JSON(http.StatusCreated, m)
}

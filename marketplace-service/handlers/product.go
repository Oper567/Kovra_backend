package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/marketplace-service/models"
	"github.com/lucepay-dev/lucepay/backend/marketplace-service/repository"
)

type ProductHandler struct {
	repo *repository.PostgresRepository
}

func NewProductHandler(repo *repository.PostgresRepository) *ProductHandler {
	return &ProductHandler{repo: repo}
}

func (h *ProductHandler) GetProducts(c *gin.Context) {
	products, err := h.repo.GetProducts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}
	// Return empty array instead of null
	if products == nil {
		products = []models.Product{}
	}
	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var p models.Product
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// In a real app, MerchantID would be derived from the auth token after verifying merchant status.
	// For now, we trust the incoming request payload if it has a merchant_id.
	if p.MerchantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "merchant_id is required"})
		return
	}

	if err := h.repo.CreateProduct(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	c.JSON(http.StatusCreated, p)
}

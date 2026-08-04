package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	httpdelivery "github.com/lucepay-dev/lucepay/backend/engagement-service/internal/delivery/http"
)

func main() {
	r := gin.Default()

	// Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "engagement-service"})
	})

	v1 := r.Group("/api/v1")
	handler := httpdelivery.NewEngagementHandler(nil) // Usecase logic omitted for brevity in stub
	handler.RegisterRoutes(v1)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	fmt.Printf("Starting engagement-service on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

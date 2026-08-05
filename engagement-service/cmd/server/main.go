package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	_ "github.com/lib/pq"
	"github.com/gin-gonic/gin"

	httpdelivery "github.com/lucepay-dev/lucepay/backend/engagement-service/internal/delivery/http"
)

func main() {
	r := gin.Default()

	// Connect to DB
	dsn := "postgres://postgres.kcxsqfbepqrcfmrefqlt:MHWDUdklbdFnU4Xw@aws-1-eu-west-2.pooler.supabase.com:5432/postgres?sslmode=require"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("Failed to open DB: %v", err)
	} else {
		if err := db.Ping(); err != nil {
			log.Printf("Failed to ping DB: %v", err)
		} else {
			log.Println("Database connected for engagement-service")
		}
	}

	// Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "engagement-service"})
	})

	v1 := r.Group("/api/v1")
	handler := httpdelivery.NewEngagementHandler(nil, db) // Usecase logic omitted for brevity in stub
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

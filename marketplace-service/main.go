package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/lucepay-dev/lucepay/backend/marketplace-service/handlers"
	"github.com/lucepay-dev/lucepay/backend/marketplace-service/repository"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres.kcxsqfbepqrcfmrefqlt:MHWDUdklbdFnU4Xw@aws-1-eu-west-2.pooler.supabase.com:5432/postgres?sslmode=require"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	repo := repository.NewPostgresRepository(db)
	if err := repo.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	productHandler := handlers.NewProductHandler(repo)
	merchantHandler := handlers.NewMerchantHandler(repo)

	r := gin.Default()

	// Add basic healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "marketplace-service"})
	})

	api := r.Group("/api/v1/marketplace")
	{
		api.GET("/products", productHandler.GetProducts)
		api.POST("/products", productHandler.CreateProduct)

		api.GET("/merchant", merchantHandler.GetMerchant)
		api.POST("/merchant", merchantHandler.CreateMerchant)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083" // Align with API Gateway EcomServiceURL
	}

	log.Printf("Marketplace service starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

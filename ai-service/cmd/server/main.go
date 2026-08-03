package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	
	"github.com/kovra-dev/kovra/backend/ai-service/internal/delivery/http"
	"github.com/kovra-dev/kovra/backend/ai-service/internal/infrastructure/gemini"
	"github.com/kovra-dev/kovra/backend/ai-service/internal/repository/postgres"
	"github.com/kovra-dev/kovra/backend/ai-service/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	// Database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	logger.Info("connected to database")

	// Gemini Provider
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		logger.Warn("GEMINI_API_KEY not set - AI features will fail")
	}
	aiProvider, err := gemini.NewGeminiProvider(context.Background(), apiKey, logger)
	if err != nil && apiKey != "" {
		log.Fatalf("failed to init gemini: %v", err)
	}

	// Repositories
	chatRepo := postgres.NewChatRepository(db)
	insightRepo := postgres.NewInsightRepository(db)
	recRepo := postgres.NewRecommendationRepository(db)

	// Usecase
	aiUsecase := usecase.NewAIUsecase(chatRepo, insightRepo, recRepo, aiProvider, logger)

	// Router
	r := gin.Default()
	
	// Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "ai-service"})
	})

	http.NewAIHandler(r, aiUsecase)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}
	
	logger.Info(fmt.Sprintf("starting kovra ai-service on :%s", port))
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

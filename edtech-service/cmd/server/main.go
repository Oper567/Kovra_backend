package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	
	"github.com/lucepay-dev/lucepay/backend/edtech-service/config"
	httpdelivery "github.com/lucepay-dev/lucepay/backend/edtech-service/internal/delivery/http"
	"github.com/lucepay-dev/lucepay/backend/edtech-service/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	logger.Info("starting lucepay edtech-service")

	cfg := config.Load()

	// ─── Usecases ───────────────────────────────────────────
	edtechUC := usecase.NewEdtechUsecase()
	quizUC := usecase.NewQuizUsecase()

	// ─── HTTP Server (Gin) ──────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "edtech-service"})
	})

	// Register routes
	v1 := router.Group("/api/v1")
	edtechHandler := httpdelivery.NewEdtechHandler(edtechUC)
	edtechHandler.RegisterRoutes(v1)

	quizHandler := httpdelivery.NewQuizHandler(quizUC)
	quizHandler.RegisterRoutes(v1)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	go func() {
		logger.Info("HTTP server starting", slog.String("port", cfg.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// ─── Graceful Shutdown ──────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutdown signal received", slog.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", slog.String("error", err.Error()))
	}
	logger.Info("lucepay edtech-service stopped gracefully")
}

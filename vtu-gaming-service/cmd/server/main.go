package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/config"
	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/domain"
	httpdelivery "github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/delivery/http"
	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/infrastructure"
	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/repository/postgres"
	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	logger.Info("starting lucepay vtu-gaming-service")

	cfg := config.Load()

	// ─── Database ───────────────────────────────────────────
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("database connected")

	// ─── Repositories ───────────────────────────────────────
	productRepo := postgres.NewProductRepo(db)
	orderRepo := postgres.NewOrderRepo(db)

	// ─── Infrastructure ───────────────────────────────────────
	walletClient := infrastructure.NewWalletHTTPClient(cfg.WalletServiceURL)
	
	providers := make(map[string]domain.ProviderAPI)
	if cfg.DatastationAPIKey != "" {
		ds := infrastructure.NewDatastationProvider(cfg.DatastationAPIKey)
		providers[ds.Name()] = ds
		logger.Info("datastation provider configured")
	} else {
		logger.Warn("DATASTATION_API_KEY is not set, VTU purchases will fail!")
	}

	// ─── Usecase ────────────────────────────────────────────
	vtuUC := usecase.NewVTUUsecase(orderRepo, productRepo, walletClient, providers, logger)

	// ─── HTTP Server (Gin) ──────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "vtu-gaming-service"})
	})

	// Register VTU routes
	v1 := router.Group("/api/v1")
	handler := httpdelivery.NewVTUHandler(vtuUC)
	handler.RegisterRoutes(v1)

	adminHandler := httpdelivery.NewAdminVTUHandler(vtuUC)
	adminHandler.RegisterRoutes(v1)

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
	logger.Info("lucepay vtu-gaming-service stopped gracefully")
}

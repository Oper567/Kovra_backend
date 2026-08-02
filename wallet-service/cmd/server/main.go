package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"github.com/kovra-dev/kovra/backend/wallet-service/config"
	httpdelivery "github.com/kovra-dev/kovra/backend/wallet-service/internal/delivery/http"
	"github.com/kovra-dev/kovra/backend/wallet-service/internal/infrastructure"
	"github.com/kovra-dev/kovra/backend/wallet-service/internal/repository/postgres"
	"github.com/kovra-dev/kovra/backend/wallet-service/internal/usecase"
)

func main() {
	// ─── Logger ──────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting kovra wallet-service")

	// ─── Configuration ──────────────────────────────────────
	cfg := config.Load()

	// ─── Database ───────────────────────────────────────────
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.Database.MaxConns)
	db.SetMaxIdleConns(cfg.Database.MinConns)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("database connected", slog.String("host", cfg.Database.Host))

	// ─── Repositories ───────────────────────────────────────
	walletRepo := postgres.NewWalletRepo(db)
	txnRepo := postgres.NewTransactionRepo(db)
	sagaRepo := postgres.NewSagaRepo(db)
	uow := postgres.NewUnitOfWork(db)

	// ─── Infrastructure ───────────────────────────────────────
	paystackClient := infrastructure.NewPaystackClient(cfg.Paystack.SecretKey)

	// ─── Usecase ────────────────────────────────────────────
	walletUC := usecase.NewWalletUsecase(walletRepo, txnRepo, sagaRepo, uow, logger)

	// ─── HTTP Server (Gin) ──────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(logger))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "wallet-service"})
	})

	// Register wallet routes
	v1 := router.Group("/api/v1")
	handler := httpdelivery.NewWalletHandler(walletUC, paystackClient)
	handler.RegisterRoutes(v1)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// ─── gRPC Server ────────────────────────────────────────
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPC.Port))
	if err != nil {
		logger.Error("failed to listen for gRPC", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// NOTE: Full gRPC server registration requires the generated protobuf code.
	// For now, we log that the gRPC listener is ready.
	logger.Info("gRPC listener ready", slog.String("port", cfg.GRPC.Port))
	_ = grpcLis // Will be used when proto is compiled

	// ─── Start HTTP Server ──────────────────────────────────
	go func() {
		logger.Info("HTTP server starting", slog.String("port", cfg.Server.Port))
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("kovra wallet-service stopped gracefully")
}

// requestLogger is a Gin middleware for structured request logging.
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		logger.Info("request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", latency),
			slog.String("ip", c.ClientIP()),
		)
	}
}

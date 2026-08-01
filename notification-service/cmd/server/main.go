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

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"github.com/kovra-dev/kovra/backend/notification-service/config"
	deliveryHttp "github.com/kovra-dev/kovra/backend/notification-service/internal/delivery/http"
	"github.com/kovra-dev/kovra/backend/notification-service/internal/delivery/rabbitmq"
	"github.com/kovra-dev/kovra/backend/notification-service/internal/infrastructure"
	"github.com/kovra-dev/kovra/backend/notification-service/internal/repository"
	"github.com/kovra-dev/kovra/backend/notification-service/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	logger.Info("starting notification-service")

	// DB
	dbDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode)
	db, err := sql.Open("postgres", dbDSN)
	if err != nil {
		logger.Error("failed to connect to db", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// Dependency Injection
	repo := repository.NewPostgresNotificationRepo(db)
	pushProvider := infrastructure.NewMockFirebaseProvider(logger)
	uc := usecase.NewNotificationUsecase(repo, pushProvider, logger)

	// RabbitMQ Consumer
	consumer, err := rabbitmq.NewConsumer(cfg.RabbitMQURL, uc, logger)
	if err == nil {
		consumer.Start()
		defer consumer.Close()
		logger.Info("rabbitmq consumer started")
	} else {
		logger.Warn("failed to start rabbitmq consumer", slog.String("error", err.Error()))
	}

	// HTTP Server
	r := gin.Default()
	handler := deliveryHttp.NewHandler(uc)
	handler.RegisterRoutes(r)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logger.Info("http server listening", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	srv.Shutdown(ctx)
}

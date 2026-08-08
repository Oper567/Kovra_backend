package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/google"
	"github.com/redis/go-redis/v9"

	"github.com/lucepay-dev/lucepay/backend/api-gateway/auth"
	"github.com/lucepay-dev/lucepay/backend/api-gateway/config"
	"github.com/lucepay-dev/lucepay/backend/api-gateway/middleware"
	"github.com/lucepay-dev/lucepay/backend/api-gateway/proxy"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	logger.Info("starting lucepay api-gateway")

	cfg := config.Load()

	// ─── Database ───────────────────────────────────────────
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
	} else {
		logger.Info("database connected", slog.String("host", cfg.DBHost))
	}

	// ─── OAuth Providers ────────────────────────────────────
	if cfg.GoogleClientID != "" {
		goth.UseProviders(
			google.New(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.OAuthCallbackURL),
		)
	}

	// ─── Redis ──────────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Warn("redis unavailable, rate limiting disabled", slog.String("error", err.Error()))
	}

	// ─── Middleware ─────────────────────────────────────────
	rateLimiter := middleware.NewRateLimiter(rdb, cfg.RateLimitPerMin)

	// Circuit breakers (one per downstream service)
	// Render cold starts take a long time, so we increase the max failures to 20
	walletCB := middleware.NewCircuitBreaker(20, 30*time.Second)
	vtuCB := middleware.NewCircuitBreaker(20, 30*time.Second)
	ecomCB := middleware.NewCircuitBreaker(20, 30*time.Second)
	edtechCB := middleware.NewCircuitBreaker(20, 30*time.Second)
	aiCB := middleware.NewCircuitBreaker(20, 30*time.Second)
	engageCB := middleware.NewCircuitBreaker(20, 30*time.Second)
	notifCB := middleware.NewCircuitBreaker(20, 30*time.Second)

	// ─── Service Proxies ────────────────────────────────────
	walletProxy := proxy.NewServiceProxy(cfg.WalletServiceURL)
	vtuProxy := proxy.NewServiceProxy(cfg.VTUServiceURL)
	ecomProxy := proxy.NewServiceProxy(cfg.EcomServiceURL)
	edtechProxy := proxy.NewServiceProxy(cfg.EdtechServiceURL)
	aiProxy := proxy.NewServiceProxy(cfg.AIServiceURL)
	engageProxy := proxy.NewServiceProxy(cfg.EngagementServiceURL)
	notifProxy := proxy.NewServiceProxy(cfg.NotificationServiceURL)

	// ─── Handlers ───────────────────────────────────────────
	oauthHandler := auth.NewOAuthHandler(db, cfg, logger)
	emailAuthHandler := auth.NewEmailAuthHandler(db, rdb, cfg, logger)

	// ─── Router ─────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "api-gateway",
			"circuits": gin.H{
				"wallet":       walletCB.State(),
				"vtu":          vtuCB.State(),
				"ecom":         ecomCB.State(),
				"edtech":       edtechCB.State(),
				"ai":           aiCB.State(),
				"engagement":   engageCB.State(),
				"notification": notifCB.State(),
			},
		})
	})

	// Root path for Render health checks
	r.HEAD("/", func(c *gin.Context) {
		c.Status(200)
	})
	r.GET("/", func(c *gin.Context) {
		c.String(200, "Luce Pay API Gateway is running")
	})

	// ─── Public Routes (no auth) ────────────────────────────
	pub := r.Group("/api/v1")
	pub.Use(middleware.OptionalJWTAuth(cfg.JWTSecret, cfg.JWTIssuer))
	pub.Use(rateLimiter.Limit())
	{
		// OAuth Routes
		pub.GET("/auth/:provider", oauthHandler.BeginAuth)
		pub.GET("/auth/:provider/callback", oauthHandler.Callback)
		pub.POST("/auth/:provider/callback", oauthHandler.TokenCallback)

		// Email Auth Routes
		pub.POST("/auth/register", emailAuthHandler.Register)
		pub.POST("/auth/verify", emailAuthHandler.Verify)
		pub.POST("/auth/login", emailAuthHandler.Login)
		// pub.POST("/auth/refresh", emailAuthHandler.Refresh) // TODO: Implement token refresh if needed

		// Public Wallet Webhooks (Paystack)
		pub.POST("/webhooks/paystack", func(c *gin.Context) {
			slog.Info("GATEWAY: Received incoming Paystack webhook", slog.String("path", c.Request.URL.Path), slog.String("method", c.Request.Method))
			c.Next()
		}, middleware.CircuitBreakerMiddleware(walletCB, "wallet-service"), walletProxy.Forward(""))
	}

	// ─── Authenticated Routes ───────────────────────────────
	auth := r.Group("/api/v1")
	auth.Use(middleware.JWTAuth(cfg.JWTSecret, cfg.JWTIssuer))
	auth.Use(rateLimiter.Limit())
	{
		// Wallet
		auth.Any("/wallet", middleware.CircuitBreakerMiddleware(walletCB, "wallet-service"), walletProxy.Forward(""))
		auth.Any("/wallet/*path", middleware.CircuitBreakerMiddleware(walletCB, "wallet-service"), walletProxy.Forward(""))

		// VTU & Gaming
		auth.Any("/vtu", middleware.CircuitBreakerMiddleware(vtuCB, "vtu-service"), vtuProxy.Forward(""))
		auth.Any("/vtu/*path", middleware.CircuitBreakerMiddleware(vtuCB, "vtu-service"), vtuProxy.Forward(""))

		// EdTech
		auth.Any("/edtech", middleware.CircuitBreakerMiddleware(edtechCB, "edtech-service"), edtechProxy.Forward(""))
		auth.Any("/edtech/*path", middleware.CircuitBreakerMiddleware(edtechCB, "edtech-service"), edtechProxy.Forward(""))

		// AI Features
		auth.Any("/ai", middleware.CircuitBreakerMiddleware(aiCB, "ai-service"), aiProxy.Forward(""))
		auth.Any("/ai/*path", middleware.CircuitBreakerMiddleware(aiCB, "ai-service"), aiProxy.Forward(""))

		// Engagement (referrals, streaks, achievements)
		auth.Any("/engage", middleware.CircuitBreakerMiddleware(engageCB, "engagement-service"), engageProxy.Forward(""))
		auth.Any("/engage/*path", middleware.CircuitBreakerMiddleware(engageCB, "engagement-service"), engageProxy.Forward(""))

		// Notifications (device registration, etc.)
		auth.Any("/notifications", middleware.CircuitBreakerMiddleware(notifCB, "notification-service"), notifProxy.Forward(""))
		auth.Any("/notifications/*path", middleware.CircuitBreakerMiddleware(notifCB, "notification-service"), notifProxy.Forward(""))

		// Rewards (routed to Engagement Service)
		auth.Any("/rewards", middleware.CircuitBreakerMiddleware(engageCB, "engagement-service"), engageProxy.Forward(""))
		auth.Any("/rewards/*path", middleware.CircuitBreakerMiddleware(engageCB, "engagement-service"), engageProxy.Forward(""))

		// User profile (local auth handler)
		auth.GET("/user/profile", emailAuthHandler.GetProfile)
		auth.PUT("/user/profile", emailAuthHandler.UpdateProfile)
		auth.POST("/user/kyc", emailAuthHandler.SubmitKyc)
	}

	// ─── Marketplace Routes (Mixed Auth) ────────────────────
	market := r.Group("/api/v1/marketplace")
	market.Use(rateLimiter.Limit())
	market.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/marketplace/products") {
			middleware.OptionalJWTAuth(cfg.JWTSecret, cfg.JWTIssuer)(c)
		} else {
			middleware.JWTAuth(cfg.JWTSecret, cfg.JWTIssuer)(c)
		}
	})
	market.Any("", middleware.CircuitBreakerMiddleware(ecomCB, "ecom-service"), ecomProxy.Forward(""))
	market.Any("/*path", middleware.CircuitBreakerMiddleware(ecomCB, "ecom-service"), ecomProxy.Forward(""))

	// ─── Start Server ───────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	go func() {
		logger.Info("API Gateway listening", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("api-gateway stopped")
}

// ─── CORS Middleware ────────────────────────────────────────

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Idempotency-Key")
		c.Header("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// ─── Auth Handlers (Inline in Gateway) ──────────────────────
// Authentication lives in the gateway since it's the entry point.

func handleRegister(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Wire to user-service or inline user creation
		c.JSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"error":   "Registration endpoint — wire to user-service",
		})
	}
}

func handleLogin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Validate credentials, issue JWT
		c.JSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"error":   "Login endpoint — wire to user-service",
		})
	}
}

func handleRefresh(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Refresh JWT token
		c.JSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"error":   "Token refresh endpoint — wire to user-service",
		})
	}
}

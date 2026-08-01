package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	JWTSecret       string
	JWTIssuer       string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	RateLimitPerMin int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	// Downstream service addresses
	WalletServiceURL    string
	VTUServiceURL       string
	EcomServiceURL      string
	EdtechServiceURL    string
	AIServiceURL        string
	EngagementServiceURL string
	NotificationServiceURL string
}

func Load() *Config {
	return &Config{
		Port:            envOr("GATEWAY_PORT", "8080"),
		JWTSecret:       envOr("JWT_SECRET", "kovra-dev-secret-change-in-production"),
		JWTIssuer:       envOr("JWT_ISSUER", "kovra-auth"),
		RedisAddr:       envOr("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   envOr("REDIS_PASSWORD", ""),
		RedisDB:         envIntOr("REDIS_DB", 0),
		RateLimitPerMin: envIntOr("RATE_LIMIT_PER_MIN", 100),
		ReadTimeout:     envDurOr("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    envDurOr("WRITE_TIMEOUT", 10*time.Second),
		ShutdownTimeout: envDurOr("SHUTDOWN_TIMEOUT", 15*time.Second),

		WalletServiceURL:     envOr("WALLET_SERVICE_URL", "http://localhost:8081"),
		VTUServiceURL:        envOr("VTU_SERVICE_URL", "http://localhost:8082"),
		EcomServiceURL:       envOr("ECOM_SERVICE_URL", "http://localhost:8083"),
		EdtechServiceURL:     envOr("EDTECH_SERVICE_URL", "http://localhost:8084"),
		AIServiceURL:         envOr("AI_SERVICE_URL", "http://localhost:8085"),
		EngagementServiceURL: envOr("ENGAGEMENT_SERVICE_URL", "http://localhost:8086"),
		NotificationServiceURL: envOr("NOTIFICATION_SERVICE_URL", "http://localhost:8087"),
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envIntOr(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return d
}

func envDurOr(k string, d time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			return dur
		}
	}
	return d
}

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

	// Database config
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// OAuth config
	GoogleClientID     string
	GoogleClientSecret string
	OAuthCallbackURL   string

	// Email config
	ResendAPIKey string
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string

	// Downstream service addresses
	WalletServiceURL       string
	VTUServiceURL          string
	EcomServiceURL         string
	EdtechServiceURL       string
	AIServiceURL           string
	EngagementServiceURL   string
	NotificationServiceURL string
}

func Load() *Config {
	return &Config{
		Port:            envOr("GATEWAY_PORT", "8080"),
		JWTSecret:       envOr("JWT_SECRET", "lucepay-dev-secret-change-in-production"),
		JWTIssuer:       envOr("JWT_ISSUER", "lucepay-auth"),
		RedisAddr:       envOr("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   envOr("REDIS_PASSWORD", ""),
		RedisDB:         envIntOr("REDIS_DB", 0),
		RateLimitPerMin: envIntOr("RATE_LIMIT_PER_MIN", 100),
		ReadTimeout:     envDurOr("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    envDurOr("WRITE_TIMEOUT", 10*time.Second),
		ShutdownTimeout: envDurOr("SHUTDOWN_TIMEOUT", 15*time.Second),

		DBHost:     envOr("DB_HOST", "aws-1-eu-west-2.pooler.supabase.com"),
		DBPort:     envOr("DB_PORT", "5432"),
		DBUser:     envOr("DB_USER", "postgres.kcxsqfbepqrcfmrefqlt"),
		DBPassword: envOr("DB_PASSWORD", "MHWDUdklbdFnU4Xw"),
		DBName:     envOr("DB_NAME", "postgres"),
		DBSSLMode:  envOr("DB_SSLMODE", "require"),

		GoogleClientID:     envOr("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: envOr("GOOGLE_CLIENT_SECRET", ""),
		OAuthCallbackURL:   envOr("OAUTH_CALLBACK_URL", "http://localhost:8080/api/v1/auth/google/callback"),

		ResendAPIKey: envOr("RESEND_API_KEY", ""),
		SMTPHost:     envOr("SMTP_HOST", ""),
		SMTPPort:     envOr("SMTP_PORT", "587"),
		SMTPUser:     envOr("SMTP_USER", ""),
		SMTPPass:     envOr("SMTP_PASS", ""),
		SMTPFrom:     envOr("SMTP_FROM", "noreply@lucepay.com"),

		WalletServiceURL:       envOr("WALLET_SERVICE_URL", "http://127.0.0.1:8081"),
		VTUServiceURL:          envOr("VTU_SERVICE_URL", "http://127.0.0.1:8082"),
		EcomServiceURL:         envOr("ECOM_SERVICE_URL", "http://127.0.0.1:8083"),
		EdtechServiceURL:       envOr("EDTECH_SERVICE_URL", "http://127.0.0.1:8084"),
		AIServiceURL:           envOr("AI_SERVICE_URL", "http://127.0.0.1:8085"),
		EngagementServiceURL:   envOr("ENGAGEMENT_SERVICE_URL", "http://127.0.0.1:8086"),
		NotificationServiceURL: envOr("NOTIFICATION_SERVICE_URL", "http://127.0.0.1:8087"),
	}
}

func (c *Config) DSN() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword + "@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName + "?sslmode=" + c.DBSSLMode
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

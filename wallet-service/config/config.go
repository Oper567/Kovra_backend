package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the wallet service.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	GRPC     GRPCConfig
	RabbitMQ RabbitMQConfig
	Paystack PaystackConfig
}

type PaystackConfig struct {
	SecretKey string
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int
	MinConns int
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type GRPCConfig struct {
	Port string
}

type RabbitMQConfig struct {
	URL string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            envOrDefault("SERVER_PORT", "8081"),
			ReadTimeout:     envDurationOrDefault("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    envDurationOrDefault("SERVER_WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: envDurationOrDefault("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefault("DB_PORT", "5432"),
			User:     envOrDefault("DB_USER", "lucepay"),
			Password: envOrDefault("DB_PASSWORD", "lucepay_secret_2024"),
			DBName:   envOrDefault("DB_NAME", "lucepay_wallet"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
			MaxConns: envIntOrDefault("DB_MAX_CONNS", 25),
			MinConns: envIntOrDefault("DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Addr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envIntOrDefault("REDIS_DB", 0),
		},
		GRPC: GRPCConfig{
			Port: envOrDefault("GRPC_PORT", "9081"),
		},
		RabbitMQ: RabbitMQConfig{
			URL: envOrDefault("RABBITMQ_URL", "amqp://lucepay:lucepay_mq_2024@localhost:5672/"),
		},
		Paystack: PaystackConfig{
			SecretKey: envOrDefault("PAYSTACK_SECRET_KEY", ""),
		},
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

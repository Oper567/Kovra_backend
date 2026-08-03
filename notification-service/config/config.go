package config

import (
	"os"
	"time"
)

type Config struct {
	Port            string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	DBSSLMode       string
	RabbitMQURL     string
	FirebaseCreds   string // Base64 encoded or path to JSON
	ShutdownTimeout time.Duration
}

func Load() *Config {
	return &Config{
		Port:            envOr("SERVER_PORT", "8087"), // Notification service port
		DBHost:          envOr("DB_HOST", "localhost"),
		DBPort:          envOr("DB_PORT", "5432"),
		DBUser:          envOr("DB_USER", "lucepay"),
		DBPassword:      envOr("DB_PASSWORD", "lucepay_secret_2024"),
		DBName:          envOr("DB_NAME", "lucepay_wallet"), // reusing main DB or a dedicated one
		DBSSLMode:       envOr("DB_SSLMODE", "disable"),
		RabbitMQURL:     envOr("RABBITMQ_URL", "amqp://lucepay:lucepay_mq_2024@localhost:5672/"),
		FirebaseCreds:   envOr("FIREBASE_CREDENTIALS", ""),
		ShutdownTimeout: 15 * time.Second,
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

package config

import (
	"os"
	"time"
)

type Config struct {
	Port              string
	DatabaseURL       string
	WalletServiceURL  string
	DatastationAPIKey string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
}

func Load() *Config {
	return &Config{
		Port:              envOrDefault("SERVER_PORT", "8082"),
		DatabaseURL:       envOrDefault("DATABASE_URL", "postgres://kovra:kovra_secret_2024@localhost:5432/kovra_wallet?sslmode=disable"),
		WalletServiceURL:  envOrDefault("WALLET_SERVICE_URL", "http://localhost:8081"),
		DatastationAPIKey: envOrDefault("DATASTATION_API_KEY", ""), // Passed via Render Env
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

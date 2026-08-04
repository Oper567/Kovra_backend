package config

import (
	"os"
	"time"
)

type Config struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

func Load() *Config {
	return &Config{
		Port:            envOr("PORT", "8084"),
		ReadTimeout:     envDurOr("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    envDurOr("WRITE_TIMEOUT", 10*time.Second),
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
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

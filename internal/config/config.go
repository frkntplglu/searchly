package config

import (
	"os"
	"time"
)

type Config struct {
	Port            string
	DatabaseURL     string
	LogLevel        string
	LogFormat       string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	Provider1URL string
	Provider2URL string
	// ProviderRateLimit is the maximum number of requests per second sent to each provider.
	ProviderRateLimit float64
	ProviderTimeout   time.Duration
	// ProviderMaxRetries is how many times a failed provider request is retried
	// (network error or timeout, 5xx, 429).
	ProviderMaxRetries int
}

func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://searchly:searchly@localhost:5432/searchly?sslmode=disable"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		LogFormat:       getEnv("LOG_FORMAT", "json"),
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 10 * time.Second,

		Provider1URL:       getEnv("PROVIDER1_URL", "https://raw.githubusercontent.com/WEG-Technology/mock/refs/heads/main/v2/provider1"),
		Provider2URL:       getEnv("PROVIDER2_URL", "https://raw.githubusercontent.com/WEG-Technology/mock/refs/heads/main/v2/provider2"),
		ProviderRateLimit:  2,
		ProviderTimeout:    5 * time.Second,
		ProviderMaxRetries: 3,
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

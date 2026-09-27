package config

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
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

	// IngestInterval is how often the ingest worker runs. Zero runs once.
	IngestInterval time.Duration
}

// Load reads the configuration from the environment. Variables in a .env file
// in the working directory are loaded first; variables already set in the
// environment take precedence over the file.
func Load() Config {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("could not load .env", "err", err)
	}

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

		IngestInterval: getDuration("INGEST_INTERVAL", 0),
	}
}

// getDuration parses a duration such as "5m". An invalid value falls back to
// the default so a typo cannot silently disable the worker.
func getDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid duration, using default", "key", key, "value", v, "default", fallback.String())
		return fallback
	}
	return d
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

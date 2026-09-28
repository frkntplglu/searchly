package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	LogLevel           string
	LogFormat          string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	ShutdownTimeout    time.Duration
	Provider1URL       string
	Provider2URL       string
	ProviderRateLimit  float64
	ProviderTimeout    time.Duration
	ProviderMaxRetries int
	IngestInterval     time.Duration
	IngestRunTimeout   time.Duration
	IngestPollInterval time.Duration
}

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

		Provider1URL: getEnv("PROVIDER1_URL", "https://raw.githubusercontent.com/WEG-Technology/mock/refs/heads/main/v2/provider1"),
		Provider2URL: getEnv("PROVIDER2_URL", "https://raw.githubusercontent.com/WEG-Technology/mock/refs/heads/main/v2/provider2"),

		// 0 means no rate limit, no timeout and no retries respectively.
		ProviderRateLimit:  getEnvAs("PROVIDER_RATE_LIMIT", 2.0, parseFloat, nonNegative),
		ProviderTimeout:    getEnvAs("PROVIDER_TIMEOUT", 5*time.Second, time.ParseDuration, nonNegative),
		ProviderMaxRetries: getEnvAs("PROVIDER_MAX_RETRIES", 3, strconv.Atoi, nonNegative),

		IngestInterval:     getEnvAs("INGEST_INTERVAL", time.Duration(0), time.ParseDuration, nonNegative),
		IngestRunTimeout:   getEnvAs("INGEST_RUN_TIMEOUT", 2*time.Minute, time.ParseDuration, positive),
		IngestPollInterval: getEnvAs("INGEST_POLL_INTERVAL", 10*time.Second, time.ParseDuration, positive),
	}
}

// getEnvAs parses the variable, falling back to the default when it is unset,
// unparsable or not valid.
func getEnvAs[T any](key string, fallback T, parse func(string) (T, error), valid func(T) bool) T {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	parsed, err := parse(v)
	if err != nil || !valid(parsed) {
		slog.Warn("invalid value, using default", "key", key, "value", v, "default", fmt.Sprint(fallback))
		return fallback
	}
	return parsed
}

type number interface {
	~int | ~int64 | ~float64
}

func nonNegative[T number](v T) bool { return v >= 0 }

func positive[T number](v T) bool { return v > 0 }

func parseFloat(s string) (float64, error) { return strconv.ParseFloat(s, 64) }

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

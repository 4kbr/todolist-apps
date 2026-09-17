// Package config membaca dan memvalidasi konfigurasi aplikasi dari
// environment variables.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config berisi seluruh konfigurasi aplikasi yang sudah divalidasi.
type Config struct {
	AppEnv             string
	HTTPPort           string
	DatabaseURL        string
	JWTSecret          string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	LogLevel           slog.Level
	CORSAllowedOrigins []string
}

// Load membaca konfigurasi dari environment variables
// fail fast jika ada vlidasi yang error
func Load() (Config, error) {
	cfg := Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		HTTPPort: getEnv("HTTP_PORT", "3020"),
	}

	// validasi  HTTP_PORT
	if cfg.HTTPPort == "" {
		return Config{}, fmt.Errorf("HTTP_PORT tidak boleh kosong")
	}

	// validasi & set DATABASE_URL
	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL tidak boleh kosong")
	}
	if err := validateDatabaseURL(databaseURL); err != nil {
		return Config{}, fmt.Errorf("DATABASE_URL invalid: %w", err)
	}
	cfg.DatabaseURL = databaseURL

	// validasi & set JWT_SECRET
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET tidak boleh kosong")
	}
	if len(jwtSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET minimal 32 karakter, sekarang: %d", len(jwtSecret))
	}
	cfg.JWTSecret = jwtSecret

	// parse ACCESS_TOKEN_TTL
	accessTokenTTL, err := mustDuration("ACCESS_TOKEN_TTL", getEnv("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return Config{}, err
	}
	cfg.AccessTokenTTL = accessTokenTTL

	// parse REFRESH_TOKEN_TTL
	refreshTokenTTL, err := mustDuration("REFRESH_TOKEN_TTL", getEnv("REFRESH_TOKEN_TTL", "720h"))
	if err != nil {
		return Config{}, err
	}
	cfg.RefreshTokenTTL = refreshTokenTTL

	// parse LOG_LEVEL
	logLevel, err := parseLogLevel(getEnv("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = logLevel

	//parse CORS_ALLOWED_ORIGINS
	corsOrigins := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	cfg.CORSAllowedOrigins = splitCSV(corsOrigins)
	return cfg, nil
}

func splitCSV(values string) []string {
	if values == "" {
		return []string{}
	}

	var result []string
	for v := range strings.SplitSeq(values, ",") {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("LOG_LEVEL invalid: %s (allowed: debug, info, warn, error)", level)
	}
}

func mustDuration(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s invalid: %w (format: 15m, 2h30m, 720h - satuan valid: ns, us, ms, s, m, h)", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s harus positive, got: %v", name, duration)
	}
	return duration, nil
}

func validateDatabaseURL(databaseURL string) error {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("schema harus postgres atau postgresql, got: %s (contoh: postgres://user:pass@host:5432/dbname)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("host tidak boleh kosomg")
	}
	return nil
}

// getEnv membaca environment variable dengan default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

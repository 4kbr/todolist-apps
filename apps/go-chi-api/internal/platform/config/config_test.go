package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("JWT_SECRET", "my-super-secret-key-at-least-32-characters-1234567890")
	t.Setenv("ACCESS_TOKEN_TTL", "15m")
	t.Setenv("REFRESH_TOKEN_TTL", "720h")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("Expected HTTPPort=8080, got: %s", cfg.HTTPPort)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("Expected AccessTokenTTL=15m, got: %v", cfg.AccessTokenTTL)
	}
}

func TestLoadMissingJWTSecret(t *testing.T) {
	os.Clearenv()
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing JWT_SECRET")
	}
}

func TestLoadJWTSecretTooShort(t *testing.T) {
	os.Clearenv()
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", "short")

	_, err := Load()

	if err == nil {
		t.Fatal("Expected error for JWT_SECRET too short")
	}
}

func TestLoadInvalidDatabaseURL(t *testing.T) {
	os.Clearenv()
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("DATABASE_URL", "mysql://localhost/db")
	t.Setenv("JWT_SECRET", "my-super-secret-key-at-least-32-characters-1234567890")

	_, err := Load()

	if err == nil {
		t.Fatal("Expected error for invalid DATABASE_URL scheme")
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	os.Clearenv()
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", "my-super-secret-key-at-least-32-characters-1234567890")
	t.Setenv("ACCESS_TOKEN_TTL", "invalid")

	_, err := Load()

	if err == nil {
		t.Fatal("Expected error for invalid duration")
	}
}

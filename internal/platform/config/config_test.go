package config

import (
	"errors"
	"testing"
)

func TestLoadPostgresConfig(t *testing.T) {
	t.Run("loads database URL and defaults", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://jungle:jungle@localhost:5433/jungle_wallet")
		t.Setenv("DATABASE_MAX_CONNECTIONS", "")
		t.Setenv("DATABASE_MIN_CONNECTIONS", "")

		config, err := LoadPostgresConfig()
		if err != nil {
			t.Fatalf("LoadPostgresConfig() unexpected error: %v", err)
		}
		if config.DatabaseURL != "postgres://jungle:jungle@localhost:5433/jungle_wallet" {
			t.Fatalf("DatabaseURL = %q", config.DatabaseURL)
		}
		if config.MaxConnections != 10 || config.MinConnections != 2 {
			t.Fatalf(
				"connections max/min = %d/%d, want 10/2",
				config.MaxConnections,
				config.MinConnections,
			)
		}
	})

	t.Run("rejects missing database URL", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")

		_, err := LoadPostgresConfig()
		if !errors.Is(err, ErrDatabaseURLRequired) {
			t.Fatalf("LoadPostgresConfig() error = %v, want ErrDatabaseURLRequired", err)
		}
	})

	t.Run("rejects minimum greater than maximum", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/database")
		t.Setenv("DATABASE_MAX_CONNECTIONS", "2")
		t.Setenv("DATABASE_MIN_CONNECTIONS", "3")

		_, err := LoadPostgresConfig()
		if !errors.Is(err, ErrInvalidConnectionLimits) {
			t.Fatalf("LoadPostgresConfig() error = %v, want ErrInvalidConnectionLimits", err)
		}
	})
}

func TestLoadHTTPConfig(t *testing.T) {
	t.Run("uses default address", func(t *testing.T) {
		t.Setenv("HTTP_ADDRESS", "")

		config, err := LoadHTTPConfig()
		if err != nil {
			t.Fatalf("LoadHTTPConfig() unexpected error: %v", err)
		}
		if config.Address != ":8080" {
			t.Fatalf("Address = %q, want :8080", config.Address)
		}
	})

	t.Run("loads configured address", func(t *testing.T) {
		t.Setenv("HTTP_ADDRESS", "127.0.0.1:9090")

		config, err := LoadHTTPConfig()
		if err != nil {
			t.Fatalf("LoadHTTPConfig() unexpected error: %v", err)
		}
		if config.Address != "127.0.0.1:9090" {
			t.Fatalf("Address = %q, want 127.0.0.1:9090", config.Address)
		}
	})
}

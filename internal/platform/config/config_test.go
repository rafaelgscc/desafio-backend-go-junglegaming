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

func TestLoadOIDCConfig(t *testing.T) {
	t.Run("loads issuer and audience", func(t *testing.T) {
		t.Setenv("OIDC_ISSUER_URL", "http://localhost:8081/realms/jungle")
		t.Setenv("OIDC_AUDIENCE", "jungle-api")

		config, err := LoadOIDCConfig()
		if err != nil {
			t.Fatalf("LoadOIDCConfig() unexpected error: %v", err)
		}
		if config.IssuerURL != "http://localhost:8081/realms/jungle" ||
			config.Audience != "jungle-api" {
			t.Fatalf("unexpected OIDC config: %#v", config)
		}
	})

	t.Run("rejects missing issuer", func(t *testing.T) {
		t.Setenv("OIDC_ISSUER_URL", "")
		t.Setenv("OIDC_AUDIENCE", "jungle-api")

		_, err := LoadOIDCConfig()
		if !errors.Is(err, ErrOIDCIssuerURLRequired) {
			t.Fatalf("LoadOIDCConfig() error = %v, want ErrOIDCIssuerURLRequired", err)
		}
	})

	t.Run("rejects missing audience", func(t *testing.T) {
		t.Setenv("OIDC_ISSUER_URL", "http://localhost:8081/realms/jungle")
		t.Setenv("OIDC_AUDIENCE", "")

		_, err := LoadOIDCConfig()
		if !errors.Is(err, ErrOIDCAudienceRequired) {
			t.Fatalf("LoadOIDCConfig() error = %v, want ErrOIDCAudienceRequired", err)
		}
	})
}

func TestLoadSQSConfig(t *testing.T) {
	t.Run("loads local SQS settings", func(t *testing.T) {
		t.Setenv("AWS_REGION", "us-east-1")
		t.Setenv("SQS_ENDPOINT", "http://localhost:4566")
		t.Setenv("SQS_QUEUE_NAME", "wager-transactions.fifo")
		t.Setenv("SQS_WORKER_ID", "worker-1")
		config, err := LoadSQSConfig()
		if err != nil {
			t.Fatal(err)
		}
		if config.Region != "us-east-1" || config.Endpoint != "http://localhost:4566" ||
			config.QueueName != "wager-transactions.fifo" || config.WorkerID != "worker-1" {
			t.Fatalf("SQS config = %#v", config)
		}
	})

	t.Run("requires a FIFO queue", func(t *testing.T) {
		t.Setenv("SQS_QUEUE_NAME", "wager-transactions")
		if _, err := LoadSQSConfig(); !errors.Is(err, ErrSQSQueueNameRequired) {
			t.Fatalf("LoadSQSConfig() error = %v", err)
		}
	})
}

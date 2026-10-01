package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/bootstrap"
)

func TestApplicationStartsAndStopsWithPostgresAndKeycloak(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	issuerURL := os.Getenv("TEST_OIDC_ISSUER_URL")
	if databaseURL == "" || issuerURL == "" {
		t.Skip("TEST_DATABASE_URL and TEST_OIDC_ISSUER_URL must be configured")
	}

	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("OIDC_ISSUER_URL", issuerURL)
	t.Setenv("OIDC_AUDIENCE", "jungle-api")
	t.Setenv("HTTP_ADDRESS", "127.0.0.1:0")

	app := fx.New(bootstrap.Module, fx.NopLogger)
	startContext, cancelStart := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStart()
	if err := app.Start(startContext); err != nil {
		t.Fatalf("start Fx application: %v", err)
	}

	stopContext, cancelStop := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStop()
	if err := app.Stop(stopContext); err != nil {
		t.Fatalf("stop Fx application: %v", err)
	}
}

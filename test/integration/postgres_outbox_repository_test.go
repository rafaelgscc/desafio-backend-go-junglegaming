package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestOutboxRepositoryAppendsIntegrationEvent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:    databaseURL,
		MaxConnections: 1,
	})
	if err != nil {
		t.Fatalf("OpenPool() unexpected error: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}
	defer conn.Release()

	schemaName := "outbox_repository_test"
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		t.Fatalf("drop test schema before test: %v", err)
	}
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE")
	}()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+quotedSchemaName); err != nil {
		t.Fatalf("set test search_path: %v", err)
	}
	if _, err := tx.Exec(ctx, readMigrationFile(t, "000004_create_outbox_events.up.sql")); err != nil {
		t.Fatalf("apply outbox migration: %v", err)
	}

	repository, err := platformpostgres.NewOutboxRepository(tx)
	if err != nil {
		t.Fatalf("NewOutboxRepository() unexpected error: %v", err)
	}

	amount, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() amount setup error: %v", err)
	}
	balance, err := domain.NewMoney("75.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() balance setup error: %v", err)
	}
	occurredAt := time.Date(2026, time.October, 1, 14, 0, 0, 0, time.UTC)
	event := application.IntegrationEvent{
		EventID:       "event-1",
		EventType:     application.IntegrationEventTypeWagerTransactionProcessed,
		AggregateID:   "transaction-1",
		CorrelationID: "correlation-1",
		CausationID:   "request-1",
		OccurredAt:    occurredAt,
		Version:       application.IntegrationEventVersion,
		Data: application.WagerTransactionProcessedData{
			TransactionID:         "transaction-1",
			ProviderID:            "provider-1",
			ExternalTransactionID: "external-1",
			Kind:                  domain.WagerTransactionKindBet,
			Money:                 amount,
			Balance:               balance,
		},
	}

	if err := repository.AppendOutboxEvent(ctx, event); err != nil {
		t.Fatalf("AppendOutboxEvent() unexpected error: %v", err)
	}

	var (
		eventType       string
		aggregateID     string
		correlationID   string
		causationID     pgtype.Text
		persistedAt     time.Time
		version         int
		payload         []byte
		publishedAt     pgtype.Timestamptz
		publishAttempts int
		lastError       pgtype.Text
	)
	err = tx.QueryRow(ctx, `
		SELECT
			event_type,
			aggregate_id,
			correlation_id,
			causation_id,
			occurred_at,
			version,
			payload,
			published_at,
			publish_attempts,
			last_error
		FROM outbox_events
		WHERE event_id = $1
	`, event.EventID).Scan(
		&eventType,
		&aggregateID,
		&correlationID,
		&causationID,
		&persistedAt,
		&version,
		&payload,
		&publishedAt,
		&publishAttempts,
		&lastError,
	)
	if err != nil {
		t.Fatalf("read persisted outbox event: %v", err)
	}

	if eventType != string(event.EventType) ||
		aggregateID != event.AggregateID ||
		correlationID != event.CorrelationID ||
		!causationID.Valid || causationID.String != event.CausationID ||
		!persistedAt.Equal(occurredAt) ||
		version != event.Version ||
		publishedAt.Valid || publishAttempts != 0 || lastError.Valid {
		t.Fatalf("persisted outbox metadata differs from event or initial publication state")
	}

	var decodedPayload map[string]any
	if err := json.Unmarshal(payload, &decodedPayload); err != nil {
		t.Fatalf("decode persisted payload: %v", err)
	}
	if decodedPayload["transactionId"] != "transaction-1" ||
		decodedPayload["providerId"] != "provider-1" ||
		decodedPayload["externalTransactionId"] != "external-1" ||
		decodedPayload["kind"] != "BET" {
		t.Fatalf("persisted payload has unexpected contract: %s", payload)
	}
	moneyPayload, ok := decodedPayload["money"].(map[string]any)
	if !ok || moneyPayload["amount"] != "25.00" || moneyPayload["currency"] != "BRL" {
		t.Fatalf("persisted money payload has unexpected contract: %s", payload)
	}

	if err := repository.AppendOutboxEvent(ctx, event); !errors.Is(
		err,
		application.ErrOutboxEventAlreadyExists,
	) {
		t.Fatalf(
			"duplicate AppendOutboxEvent() error = %v, want ErrOutboxEventAlreadyExists",
			err,
		)
	}
}

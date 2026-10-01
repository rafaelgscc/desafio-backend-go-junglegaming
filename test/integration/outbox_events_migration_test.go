package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestOutboxEventsMigrationUpAndDown(t *testing.T) {
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

	schemaName := "outbox_events_migration_test"
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

	if _, err := conn.Exec(ctx, "SET search_path TO "+quotedSchemaName); err != nil {
		t.Fatalf("set test search_path: %v", err)
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000004_create_outbox_events.up.sql")); err != nil {
		t.Fatalf("apply outbox events up migration: %v", err)
	}

	insertEvent := `
		INSERT INTO outbox_events (
			event_id,
			event_type,
			aggregate_id,
			correlation_id,
			causation_id,
			occurred_at,
			version,
			payload
		) VALUES (
			$1,
			$2,
			'transaction-1',
			'correlation-1',
			'request-1',
			NOW(),
			1,
			'{"transaction_id":"transaction-1"}'::jsonb
		)
	`

	if _, err := conn.Exec(
		ctx,
		insertEvent,
		"event-1",
		"WagerTransactionProcessed",
	); err != nil {
		t.Fatalf("insert valid outbox event: %v", err)
	}

	if _, err := conn.Exec(
		ctx,
		insertEvent,
		"event-1",
		"WagerTransactionProcessed",
	); err == nil {
		t.Fatal("duplicate event ID unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, insertEvent, "event-2", "UnknownEvent"); err == nil {
		t.Fatal("unknown event type unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO outbox_events (
			event_id, event_type, aggregate_id, correlation_id,
			occurred_at, version, payload
		) VALUES (
			'event-3', 'WalletBalanceChanged', 'wallet-1', 'correlation-1',
			NOW(), 1, '[]'::jsonb
		)
	`); err == nil {
		t.Fatal("outbox event with non-object payload unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000004_create_outbox_events.down.sql")); err != nil {
		t.Fatalf("apply outbox events down migration: %v", err)
	}

	var removedTableName *string
	if err := conn.QueryRow(ctx, "SELECT to_regclass('outbox_events')::text").Scan(&removedTableName); err != nil {
		t.Fatalf("find outbox_events after down migration: %v", err)
	}
	if removedTableName != nil {
		t.Fatalf("outbox_events still exists after down migration: %q", *removedTableName)
	}
}

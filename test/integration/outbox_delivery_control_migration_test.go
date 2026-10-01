package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestOutboxDeliveryControlMigrationUpAndDown(t *testing.T) {
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

	schemaName := "outbox_delivery_control_migration_test"
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
		t.Fatalf("apply outbox prerequisite migration: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO outbox_events (
			event_id, event_type, aggregate_id, correlation_id,
			occurred_at, version, payload
		) VALUES (
			'event-1', 'WagerTransactionProcessed', 'transaction-1', 'correlation-1',
			NOW(), 1, '{}'::jsonb
		)
	`); err != nil {
		t.Fatalf("insert event created before delivery-control migration: %v", err)
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000006_add_outbox_delivery_control.up.sql")); err != nil {
		t.Fatalf("apply outbox delivery-control up migration: %v", err)
	}

	var nextAttemptAt time.Time
	if err := conn.QueryRow(ctx, `
		SELECT next_publish_attempt_at
		FROM outbox_events
		WHERE event_id = 'event-1'
	`).Scan(&nextAttemptAt); err != nil {
		t.Fatalf("read next publication attempt: %v", err)
	}
	if nextAttemptAt.IsZero() {
		t.Fatal("existing event was not made eligible for publication")
	}

	if _, err := conn.Exec(ctx, `
		UPDATE outbox_events
		SET lease_owner = 'worker-1'
		WHERE event_id = 'event-1'
	`); err == nil {
		t.Fatal("incomplete lease unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `
		UPDATE outbox_events
		SET lease_owner = 'worker-1', lease_expires_at = NOW() + INTERVAL '30 seconds'
		WHERE event_id = 'event-1'
	`); err != nil {
		t.Fatalf("create complete lease: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = NOW()
		WHERE event_id = 'event-1'
	`); err == nil {
		t.Fatal("publishing an event without releasing its lease unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = NOW(), lease_owner = NULL, lease_expires_at = NULL
		WHERE event_id = 'event-1'
	`); err != nil {
		t.Fatalf("publish event and release lease: %v", err)
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000006_add_outbox_delivery_control.down.sql")); err != nil {
		t.Fatalf("apply outbox delivery-control down migration: %v", err)
	}

	var deliveryColumns int
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'outbox_events'
		  AND column_name IN (
		      'next_publish_attempt_at',
		      'lease_owner',
		      'lease_expires_at'
		  )
	`).Scan(&deliveryColumns); err != nil {
		t.Fatalf("count delivery-control columns after down migration: %v", err)
	}
	if deliveryColumns != 0 {
		t.Fatalf("delivery-control columns after down migration = %d, want 0", deliveryColumns)
	}

	var restoredIndex *string
	if err := conn.QueryRow(ctx, `
		SELECT to_regclass('outbox_events_unpublished_idx')::text
	`).Scan(&restoredIndex); err != nil {
		t.Fatalf("find restored unpublished index: %v", err)
	}
	if restoredIndex == nil {
		t.Fatal("original unpublished index was not restored by down migration")
	}
}

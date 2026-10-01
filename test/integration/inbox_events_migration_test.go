package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestInboxEventsMigrationUpAndDown(t *testing.T) {
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

	schemaName := "inbox_events_migration_test"
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

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000007_create_inbox_events.up.sql")); err != nil {
		t.Fatalf("apply inbox events up migration: %v", err)
	}

	insertEvent := `
		INSERT INTO inbox_events (message_id, event_id, event_type, payload)
		VALUES ($1, $2, 'WagerTransactionRequested', '{}'::jsonb)
	`
	if _, err := conn.Exec(ctx, insertEvent, "message-1", "event-1"); err != nil {
		t.Fatalf("insert valid inbox event: %v", err)
	}
	if _, err := conn.Exec(ctx, insertEvent, "message-1", "event-2"); err == nil {
		t.Fatal("duplicate message ID unexpectedly succeeded")
	}
	if _, err := conn.Exec(ctx, insertEvent, "message-2", "event-1"); err == nil {
		t.Fatal("duplicate event ID unexpectedly succeeded")
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO inbox_events (message_id, event_id, event_type, payload)
		VALUES ('message-3', 'event-3', 'WagerTransactionRequested', '[]'::jsonb)
	`); err == nil {
		t.Fatal("inbox event with non-object payload unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `
		UPDATE inbox_events SET lease_owner = 'worker-1' WHERE message_id = 'message-1'
	`); err == nil {
		t.Fatal("incomplete inbox lease unexpectedly succeeded")
	}
	if _, err := conn.Exec(ctx, `
		UPDATE inbox_events
		SET lease_owner = 'worker-1', lease_expires_at = NOW() + INTERVAL '30 seconds'
		WHERE message_id = 'message-1'
	`); err != nil {
		t.Fatalf("create complete inbox lease: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		UPDATE inbox_events SET processed_at = NOW() WHERE message_id = 'message-1'
	`); err == nil {
		t.Fatal("processing without releasing inbox lease unexpectedly succeeded")
	}
	if _, err := conn.Exec(ctx, `
		UPDATE inbox_events
		SET processed_at = NOW(), lease_owner = NULL, lease_expires_at = NULL
		WHERE message_id = 'message-1'
	`); err != nil {
		t.Fatalf("complete inbox event and release lease: %v", err)
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000007_create_inbox_events.down.sql")); err != nil {
		t.Fatalf("apply inbox events down migration: %v", err)
	}
	var removedTable *string
	if err := conn.QueryRow(ctx, "SELECT to_regclass('inbox_events')::text").Scan(&removedTable); err != nil {
		t.Fatalf("find inbox_events after down migration: %v", err)
	}
	if removedTable != nil {
		t.Fatalf("inbox_events still exists after down migration: %q", *removedTable)
	}
}

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestInboxRepositoryCoordinatesWorkersAndDeduplicatesMessages(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL: databaseURL, MaxConnections: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schemaName := "inbox_repository_test"
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE") }()
	if _, err := pool.Exec(ctx, "SET search_path TO "+quotedSchemaName); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{
		"000007_create_inbox_events.up.sql", "000009_harden_inbox_events.up.sql",
	} {
		if _, err := pool.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}
	repository, err := platformpostgres.NewInboxRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	message := application.InboxMessage{
		ConsumerName: application.WagerTransactionsConsumerName,
		MessageID:    "message-1", EventID: "event-1",
		EventType: "WagerTransactionRequested", PayloadHash: "sha256:one",
		Payload: []byte(`{"messageId":"message-1"}`), ReceivedAt: now,
	}

	claim, err := repository.Claim(ctx, message, "worker-1", now.Add(30*time.Second))
	if err != nil || claim != application.InboxClaimed {
		t.Fatalf("first Claim() = %q, %v", claim, err)
	}
	message.ReceivedAt = now.Add(time.Second)
	claim, err = repository.Claim(ctx, message, "worker-2", now.Add(31*time.Second))
	if err != nil || claim != application.InboxBusy {
		t.Fatalf("concurrent Claim() = %q, %v", claim, err)
	}
	message.ReceivedAt = now.Add(31 * time.Second)
	claim, err = repository.Claim(ctx, message, "worker-2", now.Add(time.Minute))
	if err != nil || claim != application.InboxClaimed {
		t.Fatalf("expired lease Claim() = %q, %v", claim, err)
	}
	if err := repository.Complete(
		ctx, message.ConsumerName, message.MessageID, "worker-2", now.Add(32*time.Second),
	); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	message.ReceivedAt = now.Add(33 * time.Second)
	claim, err = repository.Claim(ctx, message, "worker-3", now.Add(time.Minute))
	if err != nil || claim != application.InboxAlreadyProcessed {
		t.Fatalf("completed replay Claim() = %q, %v", claim, err)
	}
	message.PayloadHash = "sha256:different"
	if _, err := repository.Claim(ctx, message, "worker-3", now.Add(time.Minute)); !errors.Is(err, application.ErrInboxPayloadConflict) {
		t.Fatalf("changed replay error = %v", err)
	}

	var attempts int
	var processedAt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT processing_attempts, processed_at FROM inbox_events
		WHERE consumer_name = $1 AND message_id = $2
	`, message.ConsumerName, message.MessageID).Scan(&attempts, &processedAt); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || processedAt == nil {
		t.Fatalf("persisted attempts/processedAt = %d/%v", attempts, processedAt)
	}
	if _, err := pool.Exec(ctx, readMigrationFile(t, "000009_harden_inbox_events.down.sql")); err != nil {
		t.Fatalf("revert hardened inbox migration: %v", err)
	}
	var hardenedColumns int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'inbox_events'
			AND column_name IN ('consumer_name', 'payload_hash')
	`).Scan(&hardenedColumns); err != nil {
		t.Fatal(err)
	}
	if hardenedColumns != 0 {
		t.Fatalf("hardened columns after down migration = %d", hardenedColumns)
	}
}

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestWalletLedgerEntriesAppendOnlyMigrationUpAndDown(t *testing.T) {
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

	schemaName := "wallet_ledger_append_only_migration_test"
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

	for _, migration := range []string{
		"000001_create_wallets.up.sql",
		"000002_create_wager_transactions.up.sql",
		"000003_create_wallet_ledger_entries.up.sql",
	} {
		if _, err := conn.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply prerequisite migration %s: %v", migration, err)
		}
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES ('wallet-1', 'player-1', 'BRL', 7500, 2, NOW(), NOW());

		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, result_balance_in_cents, result_balance_currency, created_at, updated_at
		) VALUES (
			'transaction-1', 'external-1', 'provider-1', 'idempotency-1', 'hash-1',
			'wallet-1', 'player-1', 'round-1', 'game-1', 'BET', 2500, 'BRL',
			'PROCESSED', 7500, 'BRL', NOW(), NOW()
		);

		INSERT INTO wallet_ledger_entries (
			id, wallet_id, transaction_id, direction, amount_in_cents, currency,
			balance_before_in_cents, balance_after_in_cents, created_at
		) VALUES (
			'entry-1', 'wallet-1', 'transaction-1', 'DEBIT', 2500, 'BRL',
			10000, 7500, NOW()
		);
	`); err != nil {
		t.Fatalf("insert fixtures: %v", err)
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000005_make_wallet_ledger_entries_append_only.up.sql")); err != nil {
		t.Fatalf("apply append-only up migration: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		UPDATE wallet_ledger_entries
		SET created_at = created_at + INTERVAL '1 second'
		WHERE id = 'entry-1'
	`); err == nil {
		t.Fatal("updating a wallet ledger entry unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `
		DELETE FROM wallet_ledger_entries WHERE id = 'entry-1'
	`); err == nil {
		t.Fatal("deleting a wallet ledger entry unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `TRUNCATE wallet_ledger_entries`); err == nil {
		t.Fatal("truncating wallet ledger entries unexpectedly succeeded")
	}

	var entries int
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM wallet_ledger_entries WHERE id = 'entry-1'
	`).Scan(&entries); err != nil {
		t.Fatalf("count wallet ledger entries: %v", err)
	}
	if entries != 1 {
		t.Fatalf("wallet ledger entry count = %d, want 1", entries)
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000005_make_wallet_ledger_entries_append_only.down.sql")); err != nil {
		t.Fatalf("apply append-only down migration: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		DELETE FROM wallet_ledger_entries WHERE id = 'entry-1'
	`); err != nil {
		t.Fatalf("delete wallet ledger entry after down migration: %v", err)
	}
}

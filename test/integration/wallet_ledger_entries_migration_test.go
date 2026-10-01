package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestWalletLedgerEntriesMigrationUpAndDown(t *testing.T) {
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

	schemaName := "wallet_ledger_entries_migration_test"
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
	} {
		if _, err := conn.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply prerequisite migration %s: %v", migration, err)
		}
	}
	if _, err := conn.Exec(ctx, readMigrationFile(t, "000003_create_wallet_ledger_entries.up.sql")); err != nil {
		t.Fatalf("apply wallet ledger entries up migration: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES
			('wallet-1', 'player-1', 'BRL', 7500, 2, NOW(), NOW()),
			('wallet-2', 'player-2', 'BRL', 5000, 1, NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert wallet fixtures: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, result_balance_in_cents, result_balance_currency, created_at, updated_at
		) VALUES (
			'transaction-1', 'external-1', 'provider-1', 'idempotency-1', 'hash-1',
			'wallet-1', 'player-1', 'round-1', 'game-1', 'BET', 2500, 'BRL',
			'PROCESSED', 7500, 'BRL', NOW(), NOW()
		)
	`); err != nil {
		t.Fatalf("insert wager transaction fixture: %v", err)
	}

	insertLedgerEntry := `
		INSERT INTO wallet_ledger_entries (
			id, wallet_id, transaction_id, direction, amount_in_cents, currency,
			balance_before_in_cents, balance_after_in_cents, created_at
		) VALUES ($1, $2, 'transaction-1', 'DEBIT', 2500, 'BRL', 10000, $3, NOW())
	`

	if _, err := conn.Exec(ctx, insertLedgerEntry, "entry-3", "wallet-2", 7500); err == nil {
		t.Fatal("ledger entry associated with another wallet unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, insertLedgerEntry, "entry-1", "wallet-1", 7500); err != nil {
		t.Fatalf("insert valid wallet ledger entry: %v", err)
	}

	if _, err := conn.Exec(ctx, insertLedgerEntry, "entry-2", "wallet-1", 7500); err == nil {
		t.Fatal("second ledger entry for the same transaction unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO wallet_ledger_entries (
			id, wallet_id, transaction_id, direction, amount_in_cents, currency,
			balance_before_in_cents, balance_after_in_cents, created_at
		) VALUES (
			'entry-4', 'wallet-1', 'transaction-1', 'DEBIT', 2500, 'BRL', 10000, 8000, NOW()
		)
	`); err == nil {
		t.Fatal("ledger entry with inconsistent balance unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000003_create_wallet_ledger_entries.down.sql")); err != nil {
		t.Fatalf("apply wallet ledger entries down migration: %v", err)
	}

	var removedTableName *string
	if err := conn.QueryRow(ctx, "SELECT to_regclass('wallet_ledger_entries')::text").Scan(&removedTableName); err != nil {
		t.Fatalf("find wallet_ledger_entries after down migration: %v", err)
	}
	if removedTableName != nil {
		t.Fatalf("wallet_ledger_entries still exists after down migration: %q", *removedTableName)
	}
}

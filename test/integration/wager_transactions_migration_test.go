package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestWagerTransactionsMigrationUpAndDown(t *testing.T) {
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

	schemaName := "wager_transactions_migration_test"
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

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000001_create_wallets.up.sql")); err != nil {
		t.Fatalf("apply wallets prerequisite migration: %v", err)
	}
	if _, err := conn.Exec(ctx, readMigrationFile(t, "000002_create_wager_transactions.up.sql")); err != nil {
		t.Fatalf("apply wager transactions up migration: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES (
			'wallet-1', 'player-1', 'BRL', 10000, 1, NOW(), NOW()
		)
	`); err != nil {
		t.Fatalf("insert wallet fixture: %v", err)
	}

	insertTransaction := `
		INSERT INTO wager_transactions (
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_in_cents,
			currency,
			status,
			created_at,
			updated_at
		) VALUES (
			$1, $2, 'provider-1', 'idempotency-1', 'hash-1',
			'wallet-1', 'player-1', 'round-1', 'game-1',
			'BET', 2500, 'BRL', 'PENDING', NOW(), NOW()
		)
	`

	if _, err := conn.Exec(ctx, insertTransaction, "transaction-1", "external-1"); err != nil {
		t.Fatalf("insert valid wager transaction: %v", err)
	}

	if _, err := conn.Exec(ctx, insertTransaction, "transaction-2", "external-2"); err == nil {
		t.Fatal("duplicate provider idempotency key unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO wager_transactions (
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_in_cents,
			currency,
			status,
			created_at,
			updated_at
		) VALUES (
			'transaction-3', 'external-3', 'provider-1', 'idempotency-3', 'hash-3',
			'wallet-1', 'player-1', 'round-1', 'game-1',
			'LOSS', 100, 'BRL', 'PENDING', NOW(), NOW()
		)
	`); err == nil {
		t.Fatal("LOSS transaction with non-zero amount unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000002_create_wager_transactions.down.sql")); err != nil {
		t.Fatalf("apply wager transactions down migration: %v", err)
	}

	var removedTableName *string
	if err := conn.QueryRow(ctx, "SELECT to_regclass('wager_transactions')::text").Scan(&removedTableName); err != nil {
		t.Fatalf("find wager_transactions after down migration: %v", err)
	}
	if removedTableName != nil {
		t.Fatalf("wager_transactions still exists after down migration: %q", *removedTableName)
	}
}

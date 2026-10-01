package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestOptionalWinReferenceMigrationUpAndDown(t *testing.T) {
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

	schemaName := "win_reference_migration_test"
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
	if _, err := conn.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES ('wallet-1', 'player-1', 'BRL', 10000, 1, NOW(), NOW());
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, result_balance_in_cents, result_balance_currency, created_at, updated_at
		) VALUES (
			'bet-1', 'external-bet-1', 'provider-1', 'key-bet-1', 'hash-bet-1',
			'wallet-1', 'player-1', 'round-1', 'game-1', 'BET', 2500, 'BRL',
			'PROCESSED', 7500, 'BRL', NOW(), NOW()
		);
	`); err != nil {
		t.Fatalf("insert prerequisites: %v", err)
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000008_allow_optional_win_reference.up.sql")); err != nil {
		t.Fatalf("apply optional WIN reference up migration: %v", err)
	}

	insertWin := `
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, reference_external_transaction_id, reference_transaction_id,
			created_at, updated_at
		) VALUES (
			$1, $2, 'provider-1', $3, $4,
			'wallet-1', 'player-1', 'round-1', 'game-1', 'WIN', 5000, 'BRL',
			'PENDING', $5, $6, NOW(), NOW()
		)
	`
	if _, err := conn.Exec(ctx, insertWin,
		"win-1", "external-win-1", "key-win-1", "hash-win-1", nil, nil,
	); err != nil {
		t.Fatalf("insert WIN without reference: %v", err)
	}
	if _, err := conn.Exec(ctx, insertWin,
		"win-2", "external-win-2", "key-win-2", "hash-win-2", "external-bet-1", "bet-1",
	); err != nil {
		t.Fatalf("insert WIN with optional reference: %v", err)
	}
	if _, err := conn.Exec(ctx, insertWin,
		"win-3", "external-win-3", "key-win-3", "hash-win-3", nil, "bet-1",
	); err == nil {
		t.Fatal("WIN with internal reference but no external reference unexpectedly succeeded")
	}

	if _, err := conn.Exec(ctx, readMigrationFile(t, "000008_allow_optional_win_reference.down.sql")); err != nil {
		t.Fatalf("apply optional WIN reference down migration: %v", err)
	}
	if _, err := conn.Exec(ctx, insertWin,
		"win-4", "external-win-4", "key-win-4", "hash-win-4", "external-bet-1", "bet-1",
	); err == nil {
		t.Fatal("WIN reference unexpectedly remained allowed after down migration")
	}
}

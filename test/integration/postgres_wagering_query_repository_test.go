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

func TestWageringQueryRepositoryReadsPagesAndReconcilesWallet(t *testing.T) {
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
		t.Fatalf("OpenPool() unexpected error: %v", err)
	}
	defer pool.Close()

	schemaName := "wagering_query_repository_test"
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
		"000001_create_wallets.up.sql", "000002_create_wager_transactions.up.sql",
		"000003_create_wallet_ledger_entries.up.sql", "000005_make_wallet_ledger_entries_append_only.up.sql",
		"000008_allow_optional_win_reference.up.sql",
	} {
		if _, err := pool.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}

	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO wallets (id, player_id, currency, balance_in_cents, version, created_at, updated_at)
		VALUES ('wallet-1', 'player-1', 'BRL', 7000, 2, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert wallet fixture: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO wager_transactions (
			id, wallet_id, player_id, kind, amount_in_cents, currency, status,
			result_balance_in_cents, result_balance_currency, created_at, updated_at
		) VALUES ('transaction-opening', 'wallet-1', 'player-1', 'OPENING', 10000, 'BRL', 'PROCESSED', 10000, 'BRL', $1, $1)
	`, now); err != nil {
		t.Fatalf("insert opening transaction fixture: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, result_balance_in_cents, result_balance_currency, created_at, updated_at
		) VALUES (
			'transaction-bet', 'external-bet-1', 'provider-a', 'idem-1', 'hash-1',
			'wallet-1', 'player-1', 'round-1', 'game-1', 'BET', 3000, 'BRL',
			'PROCESSED', 7000, 'BRL', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert bet transaction fixture: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO wallet_ledger_entries (
			id, wallet_id, transaction_id, direction, amount_in_cents, currency,
			balance_before_in_cents, balance_after_in_cents, created_at
		) VALUES
			('entry-a', 'wallet-1', 'transaction-opening', 'CREDIT', 10000, 'BRL', 0, 10000, $1),
			('entry-b', 'wallet-1', 'transaction-bet', 'DEBIT', 3000, 'BRL', 10000, 7000, $1)
	`, now); err != nil {
		t.Fatalf("insert ledger fixtures: %v", err)
	}

	repository, err := platformpostgres.NewWageringQueryRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	wallet, err := repository.FindWallet(ctx, "wallet-1")
	if err != nil || wallet.Balance().Amount() != "70.00" {
		t.Fatalf("FindWallet() = balance %s, error %v", wallet.Balance().Amount(), err)
	}
	first, err := repository.ListWalletLedger(ctx, "wallet-1", nil, 1)
	if err != nil || len(first) != 1 || first[0].ID() != "entry-a" {
		t.Fatalf("first ledger page = %#v, error %v", first, err)
	}
	cursor := &application.LedgerCursor{CreatedAt: first[0].CreatedAt(), EntryID: first[0].ID()}
	second, err := repository.ListWalletLedger(ctx, "wallet-1", cursor, 1)
	if err != nil || len(second) != 1 || second[0].ID() != "entry-b" {
		t.Fatalf("second ledger page = %#v, error %v", second, err)
	}
	transaction, err := repository.FindWagerTransactionForProvider(ctx, "provider-a", "transaction-bet")
	if err != nil || transaction.ExternalTransactionID() != "external-bet-1" {
		t.Fatalf("FindWagerTransactionForProvider() = %#v, error %v", transaction, err)
	}
	if _, err := repository.FindWagerTransactionForProvider(ctx, "provider-b", "transaction-bet"); !errors.Is(err, application.ErrWagerTransactionNotFound) {
		t.Fatalf("cross-provider lookup error = %v", err)
	}
	external, err := repository.FindWagerTransactionByExternalID(ctx, "provider-a", "external-bet-1")
	if err != nil || external.ID() != "transaction-bet" {
		t.Fatalf("FindWagerTransactionByExternalID() = %#v, error %v", external, err)
	}

	result, err := repository.ReconcileWallet(ctx, "wallet-1")
	if err != nil || !result.Consistent || result.CheckedEntries != 2 || result.Difference.Amount() != "0.00" {
		t.Fatalf("consistent reconciliation = %#v, error %v", result, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE wallets SET balance_in_cents = 7100 WHERE id = 'wallet-1'"); err != nil {
		t.Fatal(err)
	}
	result, err = repository.ReconcileWallet(ctx, "wallet-1")
	if err != nil || result.Consistent || result.Difference.Amount() != "1.00" {
		t.Fatalf("inconsistent reconciliation = %#v, error %v", result, err)
	}
}

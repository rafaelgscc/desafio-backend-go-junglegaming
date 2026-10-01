package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestWalletLedgerEntryRepositoryAppendsEntry(t *testing.T) {
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

	schemaName := "wallet_ledger_entry_repository_test"
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
	for _, migration := range []string{
		"000001_create_wallets.up.sql",
		"000002_create_wager_transactions.up.sql",
		"000003_create_wallet_ledger_entries.up.sql",
	} {
		if _, err := tx.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}

	if _, err := tx.Exec(ctx, `
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
	`); err != nil {
		t.Fatalf("insert repository fixtures: %v", err)
	}

	repository, err := platformpostgres.NewWalletLedgerEntryRepository(tx)
	if err != nil {
		t.Fatalf("NewWalletLedgerEntryRepository() unexpected error: %v", err)
	}

	amount, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() amount setup error: %v", err)
	}
	balanceBefore, err := domain.NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() balance before setup error: %v", err)
	}
	balanceAfter, err := domain.NewMoney("75.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() balance after setup error: %v", err)
	}
	createdAt := time.Date(2026, time.October, 1, 13, 0, 0, 0, time.UTC)
	entry, err := domain.NewWalletLedgerEntry(
		"entry-1",
		"wallet-1",
		"transaction-1",
		domain.LedgerDirectionDebit,
		amount,
		balanceBefore,
		balanceAfter,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWalletLedgerEntry() setup error: %v", err)
	}

	if err := repository.AppendWalletLedgerEntry(ctx, entry); err != nil {
		t.Fatalf("AppendWalletLedgerEntry() unexpected error: %v", err)
	}

	var (
		walletID             string
		transactionID        string
		direction            string
		amountInCents        int64
		currency             string
		balanceBeforeInCents int64
		balanceAfterInCents  int64
		persistedCreatedAt   time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT
			wallet_id,
			transaction_id,
			direction,
			amount_in_cents,
			currency,
			balance_before_in_cents,
			balance_after_in_cents,
			created_at
		FROM wallet_ledger_entries
		WHERE id = $1
	`, entry.ID()).Scan(
		&walletID,
		&transactionID,
		&direction,
		&amountInCents,
		&currency,
		&balanceBeforeInCents,
		&balanceAfterInCents,
		&persistedCreatedAt,
	)
	if err != nil {
		t.Fatalf("read persisted ledger entry: %v", err)
	}

	if walletID != entry.WalletID() ||
		transactionID != entry.TransactionID() ||
		direction != string(entry.Direction()) ||
		amountInCents != 2500 ||
		currency != "BRL" ||
		balanceBeforeInCents != 10000 ||
		balanceAfterInCents != 7500 ||
		!persistedCreatedAt.Equal(createdAt) {
		t.Fatalf(
			"persisted ledger entry differs: wallet=%q transaction=%q direction=%q amount=%d currency=%q before=%d after=%d createdAt=%v",
			walletID,
			transactionID,
			direction,
			amountInCents,
			currency,
			balanceBeforeInCents,
			balanceAfterInCents,
			persistedCreatedAt,
		)
	}

	if err := repository.AppendWalletLedgerEntry(ctx, entry); !errors.Is(
		err,
		application.ErrWalletLedgerEntryAlreadyExists,
	) {
		t.Fatalf(
			"duplicate AppendWalletLedgerEntry() error = %v, want ErrWalletLedgerEntryAlreadyExists",
			err,
		)
	}
}

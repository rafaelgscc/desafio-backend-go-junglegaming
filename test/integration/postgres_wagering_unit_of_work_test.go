package integration_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestPostgresWageringUnitOfWorkCommitsAndRollsBack(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	adminPool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:    databaseURL,
		MaxConnections: 1,
	})
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer adminPool.Close()

	schemaName := "wagering_unit_of_work_test"
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		t.Fatalf("drop test schema before test: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer func() {
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE")
	}()

	isolatedDatabaseURL := databaseURLWithSearchPath(t, databaseURL, schemaName)
	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:    isolatedDatabaseURL,
		MaxConnections: 2,
	})
	if err != nil {
		t.Fatalf("open isolated pool: %v", err)
	}
	defer pool.Close()

	for _, migration := range []string{
		"000001_create_wallets.up.sql",
		"000002_create_wager_transactions.up.sql",
		"000003_create_wallet_ledger_entries.up.sql",
		"000004_create_outbox_events.up.sql",
	} {
		if _, err := pool.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}

	unitOfWork, err := platformpostgres.NewPostgresWageringUnitOfWork(pool)
	if err != nil {
		t.Fatalf("NewPostgresWageringUnitOfWork() unexpected error: %v", err)
	}

	now := time.Date(2026, time.October, 1, 15, 0, 0, 0, time.UTC)
	balance, err := domain.NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	committedWallet, err := domain.NewWallet("wallet-committed", "player-1", balance, now)
	if err != nil {
		t.Fatalf("NewWallet() committed setup error: %v", err)
	}

	err = unitOfWork.WithinTransaction(ctx, func(tx application.WageringTransaction) error {
		return tx.InsertWallet(ctx, committedWallet)
	})
	if err != nil {
		t.Fatalf("WithinTransaction() commit path unexpected error: %v", err)
	}

	var committedCount int
	if err := pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM wallets WHERE id = $1",
		committedWallet.ID(),
	).Scan(&committedCount); err != nil {
		t.Fatalf("count committed wallet: %v", err)
	}
	if committedCount != 1 {
		t.Fatalf("committed wallet count = %d, want 1", committedCount)
	}

	rolledBackWallet, err := domain.NewWallet("wallet-rolled-back", "player-2", balance, now)
	if err != nil {
		t.Fatalf("NewWallet() rolled back setup error: %v", err)
	}
	wantError := errors.New("force rollback")
	err = unitOfWork.WithinTransaction(ctx, func(tx application.WageringTransaction) error {
		if err := tx.InsertWallet(ctx, rolledBackWallet); err != nil {
			return err
		}
		return wantError
	})
	if !errors.Is(err, wantError) {
		t.Fatalf("WithinTransaction() rollback error = %v, want %v", err, wantError)
	}

	var rolledBackCount int
	if err := pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM wallets WHERE id = $1",
		rolledBackWallet.ID(),
	).Scan(&rolledBackCount); err != nil {
		t.Fatalf("count rolled back wallet: %v", err)
	}
	if rolledBackCount != 0 {
		t.Fatalf("rolled back wallet count = %d, want 0", rolledBackCount)
	}
}

func databaseURLWithSearchPath(t *testing.T, databaseURL, searchPath string) string {
	t.Helper()

	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	query := parsedURL.Query()
	query.Set("search_path", searchPath)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

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

func TestWalletRepositoryPersistsAndRestoresWallet(t *testing.T) {
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

	schemaName := "wallet_repository_test"
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
	if _, err := tx.Exec(ctx, readMigrationFile(t, "000001_create_wallets.up.sql")); err != nil {
		t.Fatalf("apply wallets migration: %v", err)
	}

	repository, err := platformpostgres.NewWalletRepository(tx)
	if err != nil {
		t.Fatalf("NewWalletRepository() unexpected error: %v", err)
	}

	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	initialBalance, err := domain.NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	wallet, err := domain.NewWallet("wallet-1", "player-1", initialBalance, now)
	if err != nil {
		t.Fatalf("NewWallet() setup error: %v", err)
	}

	if err := repository.InsertWallet(ctx, wallet); err != nil {
		t.Fatalf("InsertWallet() unexpected error: %v", err)
	}

	exists, err := repository.WalletExistsForPlayerAndCurrency(ctx, "player-1", "BRL")
	if err != nil {
		t.Fatalf("WalletExistsForPlayerAndCurrency() unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("WalletExistsForPlayerAndCurrency() = false, want true")
	}

	restored, err := repository.FindWalletForUpdate(ctx, wallet.ID())
	if err != nil {
		t.Fatalf("FindWalletForUpdate() unexpected error: %v", err)
	}
	if restored.ID() != wallet.ID() ||
		restored.PlayerID() != wallet.PlayerID() ||
		restored.Balance().Amount() != "100.00" ||
		restored.Currency() != "BRL" ||
		restored.Version() != 1 {
		t.Fatalf("restored wallet does not match persisted wallet: %+v", restored)
	}

	staleWallet := restored
	credit, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() credit setup error: %v", err)
	}
	if err := restored.Credit(credit, now.Add(time.Second)); err != nil {
		t.Fatalf("Credit() setup error: %v", err)
	}
	if err := repository.SaveWallet(ctx, restored); err != nil {
		t.Fatalf("SaveWallet() unexpected error: %v", err)
	}

	updated, err := repository.FindWalletForUpdate(ctx, wallet.ID())
	if err != nil {
		t.Fatalf("FindWalletForUpdate() after update unexpected error: %v", err)
	}
	if updated.Balance().Amount() != "125.00" || updated.Version() != 2 {
		t.Fatalf(
			"updated wallet balance/version = %s/%d, want 125.00/2",
			updated.Balance().Amount(),
			updated.Version(),
		)
	}

	if err := staleWallet.Credit(credit, now.Add(2*time.Second)); err != nil {
		t.Fatalf("Credit() stale setup error: %v", err)
	}
	if err := repository.SaveWallet(ctx, staleWallet); !errors.Is(err, application.ErrConcurrentWalletUpdate) {
		t.Fatalf("SaveWallet() stale error = %v, want ErrConcurrentWalletUpdate", err)
	}

	if _, err := repository.FindWalletForUpdate(ctx, "missing-wallet"); !errors.Is(err, application.ErrWalletNotFound) {
		t.Fatalf("FindWalletForUpdate() missing error = %v, want ErrWalletNotFound", err)
	}

	if err := repository.InsertWallet(ctx, wallet); !errors.Is(err, application.ErrWalletAlreadyExists) {
		t.Fatalf("InsertWallet() duplicate error = %v, want ErrWalletAlreadyExists", err)
	}
}

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

func TestWagerTransactionRepositoryPersistsAndRestoresTransactions(t *testing.T) {
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

	schemaName := "wager_transaction_repository_test"
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
	} {
		if _, err := tx.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES ('wallet-1', 'player-1', 'BRL', 10000, 1, NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert wallet fixture: %v", err)
	}

	repository, err := platformpostgres.NewWagerTransactionRepository(tx)
	if err != nil {
		t.Fatalf("NewWagerTransactionRepository() unexpected error: %v", err)
	}

	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	amount, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() amount setup error: %v", err)
	}
	bet, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID:                    "transaction-bet-1",
		ExternalTransactionID: "external-bet-1",
		ProviderID:            "provider-1",
		IdempotencyKey:        "idempotency-bet-1",
		PayloadHash:           "hash-bet-1",
		WalletID:              "wallet-1",
		PlayerID:              "player-1",
		RoundID:               "round-1",
		GameID:                "game-1",
		Kind:                  domain.WagerTransactionKindBet,
		Money:                 amount,
		OccurredAt:            now,
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() BET setup error: %v", err)
	}
	if err := repository.InsertWagerTransaction(ctx, bet); err != nil {
		t.Fatalf("InsertWagerTransaction() BET unexpected error: %v", err)
	}

	findCases := []struct {
		name string
		find func() (domain.WagerTransaction, error)
	}{
		{
			name: "by ID",
			find: func() (domain.WagerTransaction, error) {
				return repository.FindWagerTransactionForUpdate(ctx, bet.ID())
			},
		},
		{
			name: "by idempotency key",
			find: func() (domain.WagerTransaction, error) {
				return repository.FindWagerTransactionByIdempotencyKeyForUpdate(
					ctx,
					bet.ProviderID(),
					bet.IdempotencyKey(),
				)
			},
		},
		{
			name: "by external ID",
			find: func() (domain.WagerTransaction, error) {
				return repository.FindWagerTransactionByExternalIDForUpdate(
					ctx,
					bet.ProviderID(),
					bet.ExternalTransactionID(),
				)
			},
		},
	}
	for _, testCase := range findCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := testCase.find()
			if err != nil {
				t.Fatalf("find unexpected error: %v", err)
			}
			if got.ID() != bet.ID() || got.Status() != domain.WagerTransactionStatusPending {
				t.Fatalf("find returned ID/status = %q/%q", got.ID(), got.Status())
			}
		})
	}

	resultBalance, err := domain.NewMoney("75.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() result balance setup error: %v", err)
	}
	if err := bet.MarkProcessed(resultBalance, now.Add(time.Second)); err != nil {
		t.Fatalf("MarkProcessed() setup error: %v", err)
	}
	if err := repository.SaveWagerTransaction(ctx, bet); err != nil {
		t.Fatalf("SaveWagerTransaction() unexpected error: %v", err)
	}

	restoredBet, err := repository.FindWagerTransactionForUpdate(ctx, bet.ID())
	if err != nil {
		t.Fatalf("FindWagerTransactionForUpdate() after save unexpected error: %v", err)
	}
	restoredBalance, hasBalance := restoredBet.ResultBalance()
	if restoredBet.Status() != domain.WagerTransactionStatusProcessed ||
		!hasBalance || restoredBalance.Amount() != "75.00" {
		t.Fatalf(
			"restored BET status/balance = %q/%s/%t",
			restoredBet.Status(),
			restoredBalance.Amount(),
			hasBalance,
		)
	}

	refund, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID:                             "transaction-refund-1",
		ExternalTransactionID:          "external-refund-1",
		ProviderID:                     "provider-1",
		IdempotencyKey:                 "idempotency-refund-1",
		PayloadHash:                    "hash-refund-1",
		WalletID:                       "wallet-1",
		PlayerID:                       "player-1",
		RoundID:                        "round-1",
		GameID:                         "game-1",
		Kind:                           domain.WagerTransactionKindRefund,
		Money:                          amount,
		ReferenceExternalTransactionID: bet.ExternalTransactionID(),
		OccurredAt:                     now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() REFUND setup error: %v", err)
	}
	if err := refund.ResolveReference(bet.ID(), now.Add(3*time.Second)); err != nil {
		t.Fatalf("ResolveReference() setup error: %v", err)
	}
	refundedBalance, err := domain.NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() refunded balance setup error: %v", err)
	}
	if err := refund.MarkProcessed(refundedBalance, now.Add(4*time.Second)); err != nil {
		t.Fatalf("MarkProcessed() REFUND setup error: %v", err)
	}
	if err := repository.InsertWagerTransaction(ctx, refund); err != nil {
		t.Fatalf("InsertWagerTransaction() REFUND unexpected error: %v", err)
	}

	hasProcessedRefund, err := repository.HasProcessedReversal(
		ctx,
		bet.ID(),
		domain.WagerTransactionKindRefund,
	)
	if err != nil {
		t.Fatalf("HasProcessedReversal() unexpected error: %v", err)
	}
	if !hasProcessedRefund {
		t.Fatal("HasProcessedReversal() = false, want true")
	}

	if _, err := repository.FindWagerTransactionForUpdate(ctx, "missing-transaction"); !errors.Is(err, application.ErrWagerTransactionNotFound) {
		t.Fatalf("missing transaction error = %v, want ErrWagerTransactionNotFound", err)
	}
}

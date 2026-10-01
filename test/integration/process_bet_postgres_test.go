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

func TestProcessBetWithPostgresCommitsAndRollsBackAtomically(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	adminPool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:    databaseURL,
		MaxConnections: 1,
	})
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer adminPool.Close()

	schemaName := "process_bet_postgres_test"
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

	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:    databaseURLWithSearchPath(t, databaseURL, schemaName),
		MaxConnections: 4,
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
	processBet, err := application.NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	now := time.Date(2026, time.October, 1, 16, 0, 0, 0, time.UTC)
	initialBalance, err := domain.NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() balance setup error: %v", err)
	}
	betAmount, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() bet setup error: %v", err)
	}

	createPendingBet := func(
		walletID string,
		playerID string,
		transactionID string,
		externalID string,
		idempotencyKey string,
	) {
		t.Helper()

		wallet, err := domain.NewWallet(walletID, playerID, initialBalance, now)
		if err != nil {
			t.Fatalf("NewWallet() setup error: %v", err)
		}
		transaction, err := domain.NewExternalWagerTransaction(
			domain.NewExternalWagerTransactionParams{
				ID:                    transactionID,
				ExternalTransactionID: externalID,
				ProviderID:            "provider-1",
				IdempotencyKey:        idempotencyKey,
				PayloadHash:           "hash-" + transactionID,
				WalletID:              walletID,
				PlayerID:              playerID,
				RoundID:               "round-" + transactionID,
				GameID:                "game-1",
				Kind:                  domain.WagerTransactionKindBet,
				Money:                 betAmount,
				OccurredAt:            now,
			},
		)
		if err != nil {
			t.Fatalf("NewExternalWagerTransaction() setup error: %v", err)
		}

		if err := unitOfWork.WithinTransaction(
			ctx,
			func(tx application.WageringTransaction) error {
				if err := tx.InsertWallet(ctx, wallet); err != nil {
					return err
				}
				return tx.InsertWagerTransaction(ctx, transaction)
			},
		); err != nil {
			t.Fatalf("persist wallet and pending BET fixtures: %v", err)
		}
	}

	t.Run("commits every side effect", func(t *testing.T) {
		createPendingBet(
			"wallet-success",
			"player-success",
			"transaction-success",
			"external-success",
			"idempotency-success",
		)

		result, err := processBet.Execute(ctx, application.ProcessBetCommand{
			TransactionID:         "transaction-success",
			LedgerEntryID:         "ledger-success",
			OutcomeEventID:        "event-outcome-success",
			BalanceChangedEventID: "event-balance-success",
			CorrelationID:         "correlation-success",
			CausationID:           "request-success",
			ProcessedAt:           now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("ProcessBetUseCase.Execute() unexpected error: %v", err)
		}
		if result.Status != domain.WagerTransactionStatusProcessed ||
			result.Balance.Amount() != "75.00" || result.WalletVersion != 2 {
			t.Fatalf(
				"result status/balance/version = %q/%s/%d",
				result.Status,
				result.Balance.Amount(),
				result.WalletVersion,
			)
		}

		var (
			balanceInCents int64
			walletVersion  int64
			status         string
			resultInCents  int64
			ledgerCount    int
			outboxCount    int
		)
		if err := pool.QueryRow(ctx, `
			SELECT balance_in_cents, version
			FROM wallets
			WHERE id = 'wallet-success'
		`).Scan(&balanceInCents, &walletVersion); err != nil {
			t.Fatalf("read committed wallet: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT status, result_balance_in_cents
			FROM wager_transactions
			WHERE id = 'transaction-success'
		`).Scan(&status, &resultInCents); err != nil {
			t.Fatalf("read committed wager transaction: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM wallet_ledger_entries
			WHERE transaction_id = 'transaction-success'
		`).Scan(&ledgerCount); err != nil {
			t.Fatalf("count committed ledger entries: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM outbox_events
			WHERE correlation_id = 'correlation-success'
		`).Scan(&outboxCount); err != nil {
			t.Fatalf("count committed outbox events: %v", err)
		}

		if balanceInCents != 7500 || walletVersion != 2 ||
			status != "PROCESSED" || resultInCents != 7500 ||
			ledgerCount != 1 || outboxCount != 2 {
			t.Fatalf(
				"committed state = balance %d, version %d, status %s, result %d, ledger %d, outbox %d",
				balanceInCents,
				walletVersion,
				status,
				resultInCents,
				ledgerCount,
				outboxCount,
			)
		}
	})

	t.Run("rolls back every side effect", func(t *testing.T) {
		createPendingBet(
			"wallet-rollback",
			"player-rollback",
			"transaction-rollback",
			"external-rollback",
			"idempotency-rollback",
		)

		seedEvent := application.IntegrationEvent{
			EventID:       "event-outcome-duplicate",
			EventType:     application.IntegrationEventTypeWagerTransactionProcessed,
			AggregateID:   "transaction-rollback",
			CorrelationID: "correlation-seed",
			CausationID:   "request-seed",
			OccurredAt:    now,
			Version:       application.IntegrationEventVersion,
			Data:          map[string]string{"seed": "duplicate"},
		}
		if err := unitOfWork.WithinTransaction(
			ctx,
			func(tx application.WageringTransaction) error {
				return tx.AppendOutboxEvent(ctx, seedEvent)
			},
		); err != nil {
			t.Fatalf("seed duplicate outbox event: %v", err)
		}

		_, err := processBet.Execute(ctx, application.ProcessBetCommand{
			TransactionID:         "transaction-rollback",
			LedgerEntryID:         "ledger-rollback",
			OutcomeEventID:        seedEvent.EventID,
			BalanceChangedEventID: "event-balance-rollback",
			CorrelationID:         "correlation-rollback",
			CausationID:           "request-rollback",
			ProcessedAt:           now.Add(time.Second),
		})
		if !errors.Is(err, application.ErrOutboxEventAlreadyExists) {
			t.Fatalf(
				"ProcessBetUseCase.Execute() error = %v, want ErrOutboxEventAlreadyExists",
				err,
			)
		}

		var (
			balanceInCents int64
			walletVersion  int64
			status         string
			ledgerCount    int
			balanceEvents  int
		)
		if err := pool.QueryRow(ctx, `
			SELECT balance_in_cents, version
			FROM wallets
			WHERE id = 'wallet-rollback'
		`).Scan(&balanceInCents, &walletVersion); err != nil {
			t.Fatalf("read rolled back wallet: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT status
			FROM wager_transactions
			WHERE id = 'transaction-rollback'
		`).Scan(&status); err != nil {
			t.Fatalf("read rolled back wager transaction: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM wallet_ledger_entries
			WHERE transaction_id = 'transaction-rollback'
		`).Scan(&ledgerCount); err != nil {
			t.Fatalf("count rolled back ledger entries: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM outbox_events
			WHERE event_id = 'event-balance-rollback'
		`).Scan(&balanceEvents); err != nil {
			t.Fatalf("count rolled back balance events: %v", err)
		}

		if balanceInCents != 10000 || walletVersion != 1 ||
			status != "PENDING" || ledgerCount != 0 || balanceEvents != 0 {
			t.Fatalf(
				"rolled back state = balance %d, version %d, status %s, ledger %d, balance events %d",
				balanceInCents,
				walletVersion,
				status,
				ledgerCount,
				balanceEvents,
			)
		}
	})
}

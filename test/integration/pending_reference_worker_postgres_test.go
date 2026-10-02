package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestPendingReferenceWorkerProcessesAndExpiresReversals(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL: databaseURL, MaxConnections: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schemaName := "pending_reference_worker_test"
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE")
	}()
	if _, err := pool.Exec(ctx, "SET search_path TO "+quotedSchemaName); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{
		"000001_create_wallets.up.sql",
		"000002_create_wager_transactions.up.sql",
		"000003_create_wallet_ledger_entries.up.sql",
		"000004_create_outbox_events.up.sql",
		"000005_make_wallet_ledger_entries_append_only.up.sql",
		"000008_allow_optional_win_reference.up.sql",
		"000010_add_reference_retry_leases.up.sql",
	} {
		if _, err := pool.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}

	now := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES ('wallet-1', 'player-1', 'BRL', 7000, 1, $1, $2)
	`, now.Add(-2*time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, result_balance_in_cents, result_balance_currency,
			created_at, updated_at
		) VALUES (
			'bet-1', 'external-bet-1', 'provider-1', 'bet-key-1', 'bet-hash-1',
			'wallet-1', 'player-1', 'round-1', 'game-1', 'BET', 3000, 'BRL',
			'PROCESSED', 7000, 'BRL', $1, $2
		)
	`, now.Add(-2*time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	insertPendingReferenceFixture(
		t, ctx, pool, "refund-1", "external-refund-1", "external-bet-1",
		domain.WagerTransactionKindRefund, 1, now,
	)

	repository, err := platformpostgres.NewPendingReferenceRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	unitOfWork, err := platformpostgres.NewPostgresWageringUnitOfWork(pool)
	if err != nil {
		t.Fatal(err)
	}
	refund, _ := application.NewProcessRefundUseCase(unitOfWork)
	rollback, _ := application.NewProcessRollbackUseCase(unitOfWork)
	expirer, _ := application.NewExpirePendingReferenceUseCase(unitOfWork)
	worker, err := application.NewRetryPendingReferencesUseCase(
		repository, refund, rollback, expirer,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := worker.Execute(ctx, application.RetryPendingReferencesCommand{
		WorkerID: "reference-1", Now: now, LeaseDuration: 30 * time.Second,
		RetryBaseDelay: time.Minute, MaxAttempts: 5, BatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != 1 || result.Claimed != 1 {
		t.Fatalf("processed result = %#v", result)
	}
	var balance int64
	var status string
	var referenceID *string
	if err := pool.QueryRow(ctx, `SELECT balance_in_cents FROM wallets WHERE id = 'wallet-1'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT status, reference_transaction_id
		FROM wager_transactions WHERE id = 'refund-1'
	`).Scan(&status, &referenceID); err != nil {
		t.Fatal(err)
	}
	if balance != 10000 || status != string(domain.WagerTransactionStatusProcessed) ||
		referenceID == nil || *referenceID != "bet-1" {
		t.Fatalf("balance=%d status=%s reference=%v", balance, status, referenceID)
	}
	var ledgerCount, outboxCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM wallet_ledger_entries WHERE transaction_id = 'refund-1'`).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM outbox_events
		WHERE correlation_id = 'reference-retry:refund-1'
	`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 || outboxCount != 2 {
		t.Fatalf("ledger entries=%d outbox events=%d", ledgerCount, outboxCount)
	}

	expiryTime := now.Add(time.Minute)
	insertPendingReferenceFixture(
		t, ctx, pool, "rollback-expired", "external-rollback-expired", "never-arrived",
		domain.WagerTransactionKindRollback, 5, expiryTime,
	)
	expired, err := worker.Execute(ctx, application.RetryPendingReferencesCommand{
		WorkerID: "reference-1", Now: expiryTime, LeaseDuration: 30 * time.Second,
		RetryBaseDelay: time.Minute, MaxAttempts: 5, BatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if expired.Rejected != 1 || expired.Claimed != 1 {
		t.Fatalf("expired result = %#v", expired)
	}
	var failureCode *string
	if err := pool.QueryRow(ctx, `
		SELECT status, failure_code FROM wager_transactions WHERE id = 'rollback-expired'
	`).Scan(&status, &failureCode); err != nil {
		t.Fatal(err)
	}
	if status != string(domain.WagerTransactionStatusRejected) || failureCode == nil ||
		*failureCode != string(domain.WagerTransactionFailureCodeReferenceNotFound) {
		t.Fatalf("expired status=%s failureCode=%v", status, failureCode)
	}
}

func insertPendingReferenceFixture(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	id string,
	externalID string,
	referenceExternalID string,
	kind domain.WagerTransactionKind,
	attempts int,
	now time.Time,
) {
	t.Helper()
	_, err := db.Exec(ctx, `
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, reference_external_transaction_id, reference_attempts,
			next_reference_attempt_at, created_at, updated_at
		) VALUES (
			$1, $2, 'provider-1', $3, $4,
			'wallet-1', 'player-1', 'round-1', 'game-1', $5, 3000, 'BRL',
			'PENDING_REFERENCE', $6, $7, $8, $9, $10
		)
	`, id, externalID, id+"-key", id+"-hash", string(kind), referenceExternalID,
		attempts, now, now.Add(-2*time.Hour), now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
}

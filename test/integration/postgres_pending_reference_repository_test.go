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

func TestPendingReferenceRepositoryLeasesAndReschedulesDurably(t *testing.T) {
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
		t.Fatal(err)
	}
	defer pool.Close()
	schemaName := "pending_reference_repository_test"
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
		) VALUES ('wallet-1', 'player-1', 'BRL', 10000, 1, $1, $1)
	`, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, reference_external_transaction_id, reference_attempts,
			next_reference_attempt_at, created_at, updated_at
		) VALUES (
			'refund-1', 'external-refund-1', 'provider-1', 'key-1', 'hash-1',
			'wallet-1', 'player-1', 'round-1', 'game-1', 'REFUND', 3000, 'BRL',
			'PENDING_REFERENCE', 'missing-bet', 1, $1, $2, $3
		)
	`, now, now.Add(-time.Hour), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	repository, err := platformpostgres.NewPendingReferenceRepository(pool)
	if err != nil {
		t.Fatal(err)
	}

	claimed, err := repository.ClaimPendingReferences(
		ctx, "worker-1", now, now.Add(30*time.Second), 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].TransactionID != "refund-1" ||
		claimed[0].Kind != domain.WagerTransactionKindRefund || claimed[0].ReferenceAttempts != 1 {
		t.Fatalf("claimed = %#v", claimed)
	}
	competing, err := repository.ClaimPendingReferences(
		ctx, "worker-2", now, now.Add(30*time.Second), 10,
	)
	if err != nil || len(competing) != 0 {
		t.Fatalf("competing claim = %#v, error %v", competing, err)
	}
	if err := repository.ReleasePendingReference(
		ctx, "refund-1", "worker-2",
	); !errors.Is(err, application.ErrReferenceLeaseLost) {
		t.Fatalf("wrong owner release error = %v", err)
	}

	reclaimed, err := repository.ClaimPendingReferences(
		ctx, "worker-2", now.Add(31*time.Second), now.Add(time.Minute), 10,
	)
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("expired lease claim = %#v, error %v", reclaimed, err)
	}
	nextAttemptAt := now.Add(2 * time.Minute)
	if err := repository.FailPendingReference(
		ctx, "refund-1", "worker-2", nextAttemptAt, "temporary database error",
	); err != nil {
		t.Fatal(err)
	}
	early, err := repository.ClaimPendingReferences(
		ctx, "worker-3", now.Add(time.Minute), now.Add(90*time.Second), 10,
	)
	if err != nil || len(early) != 0 {
		t.Fatalf("early retry claim = %#v, error %v", early, err)
	}
	due, err := repository.ClaimPendingReferences(
		ctx, "worker-3", nextAttemptAt, nextAttemptAt.Add(30*time.Second), 10,
	)
	if err != nil || len(due) != 1 {
		t.Fatalf("due retry claim = %#v, error %v", due, err)
	}
	if err := repository.ReleasePendingReference(ctx, "refund-1", "worker-3"); err != nil {
		t.Fatal(err)
	}

	var owner *string
	var expiresAt *time.Time
	var lastError *string
	if err := pool.QueryRow(ctx, `
		SELECT reference_lease_owner, reference_lease_expires_at, reference_last_error
		FROM wager_transactions WHERE id = 'refund-1'
	`).Scan(&owner, &expiresAt, &lastError); err != nil {
		t.Fatal(err)
	}
	if owner != nil || expiresAt != nil || lastError != nil {
		t.Fatalf("lease not cleared: owner=%v expiresAt=%v error=%v", owner, expiresAt, lastError)
	}
}

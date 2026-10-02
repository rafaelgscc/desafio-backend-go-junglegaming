package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestWagerMessageAtomicallyCompletesFinancialStateAndInbox(t *testing.T) {
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
	schemaName := "wager_message_postgres_test"
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
		"000003_create_wallet_ledger_entries.up.sql", "000004_create_outbox_events.up.sql",
		"000005_make_wallet_ledger_entries_append_only.up.sql", "000006_add_outbox_delivery_control.up.sql",
		"000007_create_inbox_events.up.sql", "000008_allow_optional_win_reference.up.sql",
		"000009_harden_inbox_events.up.sql",
	} {
		if _, err := pool.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}
	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO wallets (id, player_id, currency, balance_in_cents, version, created_at, updated_at)
		VALUES ('wallet-1', 'player-1', 'BRL', 10000, 1, $1, $1)
	`, now); err != nil {
		t.Fatal(err)
	}

	unitOfWork, _ := platformpostgres.NewPostgresWageringUnitOfWork(pool)
	submit, _ := application.NewSubmitWagerTransactionUseCase(unitOfWork)
	bet, _ := application.NewProcessBetUseCase(unitOfWork)
	win, _ := application.NewProcessWinUseCase(unitOfWork)
	loss, _ := application.NewProcessLossUseCase(unitOfWork)
	refund, _ := application.NewProcessRefundUseCase(unitOfWork)
	rollback, _ := application.NewProcessRollbackUseCase(unitOfWork)
	executor, _ := application.NewExecuteWagerTransactionUseCase(submit, bet, win, loss, refund, rollback)
	inbox, _ := platformpostgres.NewInboxRepository(pool)
	consumer, _ := application.NewConsumeWagerMessageUseCase(inbox, executor)
	body := []byte(`{
		"messageId":"message-1","type":"WagerTransactionRequested",
		"occurredAt":"2026-10-01T12:00:00Z",
		"data":{"providerId":"provider-a","externalTransactionId":"external-1",
		"idempotencyKey":"provider-a:external-1","playerId":"player-1",
		"walletId":"wallet-1","roundId":"round-1","gameId":"game-1","kind":"BET",
		"money":{"amount":"25.00","currency":"BRL"}}
	}`)
	payloadHash := sha256.Sum256(body)
	claim, err := inbox.Claim(ctx, application.InboxMessage{
		ConsumerName: application.WagerTransactionsConsumerName,
		MessageID:    "message-1", EventID: "message-1",
		EventType:   "WagerTransactionRequested",
		PayloadHash: "sha256:" + hex.EncodeToString(payloadHash[:]), Payload: body,
		ReceivedAt: now.Add(time.Second),
	}, "crashed-worker", now.Add(31*time.Second))
	if err != nil || claim != application.InboxClaimed {
		t.Fatalf("crashed worker claim = %q, error %v", claim, err)
	}
	busy, err := consumer.Execute(
		ctx, body, "worker-1", now.Add(2*time.Second),
		now.Add(32*time.Second), now.Add(3*time.Second),
	)
	if !errors.Is(err, application.ErrInboxMessageBusy) || busy.DeleteFromQueue {
		t.Fatalf("busy consume result = %#v, error = %v", busy, err)
	}

	result, err := consumer.Execute(
		ctx, body, "worker-1", now.Add(32*time.Second),
		now.Add(62*time.Second), now.Add(33*time.Second),
	)
	if err != nil || !result.DeleteFromQueue {
		t.Fatalf("first consume result = %#v, error = %v", result, err)
	}
	var balance int64
	var ledgerCount, outboxCount int
	var processed bool
	if err := pool.QueryRow(ctx, "SELECT balance_in_cents FROM wallets WHERE id = 'wallet-1'").Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM wallet_ledger_entries").Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events").Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT processed_at IS NOT NULL FROM inbox_events
		WHERE consumer_name = 'wager-transactions' AND message_id = 'message-1'
	`).Scan(&processed); err != nil {
		t.Fatal(err)
	}
	if balance != 7500 || ledgerCount != 1 || outboxCount != 2 || !processed {
		t.Fatalf("state = balance %d ledger %d outbox %d processed %v", balance, ledgerCount, outboxCount, processed)
	}

	replay, err := consumer.Execute(
		ctx, body, "worker-2", now.Add(2*time.Minute),
		now.Add(150*time.Second), now.Add(121*time.Second),
	)
	if err != nil || !replay.DeleteFromQueue || !replay.AlreadyProcessed {
		t.Fatalf("replay result = %#v, error = %v", replay, err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM wallet_ledger_entries").Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 {
		t.Fatalf("ledger count after replay = %d", ledgerCount)
	}
}

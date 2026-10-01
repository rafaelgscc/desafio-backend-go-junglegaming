package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	httpadapter "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/http"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestWagerTransactionHTTPPersistsBetAndReplaysOriginalResult(t *testing.T) {
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
	schemaName := "wager_transaction_http_postgres_test"
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		t.Fatalf("drop test schema before test: %v", err)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE")
	}()
	if _, err := pool.Exec(ctx, "SET search_path TO "+quotedSchemaName); err != nil {
		t.Fatalf("set pool search_path: %v", err)
	}
	for _, migration := range []string{
		"000001_create_wallets.up.sql",
		"000002_create_wager_transactions.up.sql",
		"000003_create_wallet_ledger_entries.up.sql",
		"000004_create_outbox_events.up.sql",
		"000005_make_wallet_ledger_entries_append_only.up.sql",
		"000006_add_outbox_delivery_control.up.sql",
		"000007_create_inbox_events.up.sql",
		"000008_allow_optional_win_reference.up.sql",
	} {
		if _, err := pool.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES ('wallet-1', 'player-1', 'BRL', 10000, 1, NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert wallet fixture: %v", err)
	}

	unitOfWork, err := platformpostgres.NewPostgresWageringUnitOfWork(pool)
	if err != nil {
		t.Fatalf("NewPostgresWageringUnitOfWork() unexpected error: %v", err)
	}
	submit, _ := application.NewSubmitWagerTransactionUseCase(unitOfWork)
	bet, _ := application.NewProcessBetUseCase(unitOfWork)
	win, _ := application.NewProcessWinUseCase(unitOfWork)
	loss, _ := application.NewProcessLossUseCase(unitOfWork)
	refund, _ := application.NewProcessRefundUseCase(unitOfWork)
	rollback, _ := application.NewProcessRollbackUseCase(unitOfWork)
	execute, err := application.NewExecuteWagerTransactionUseCase(
		submit, bet, win, loss, refund, rollback,
	)
	if err != nil {
		t.Fatalf("NewExecuteWagerTransactionUseCase() unexpected error: %v", err)
	}
	handler, err := httpadapter.NewWagerTransactionHandler(
		execute,
		&integrationSequenceIDGenerator{},
		integrationFixedClock{now: time.Date(2026, time.October, 1, 20, 0, 0, 0, time.UTC)},
	)
	if err != nil {
		t.Fatalf("NewWagerTransactionHandler() unexpected error: %v", err)
	}

	requestBody := `{
		"providerId":"provider-a","externalTransactionId":"external-bet-1",
		"playerId":"player-1","walletId":"wallet-1","roundId":"round-1",
		"gameId":"game-1","kind":"BET",
		"money":{"amount":"25.00","currency":"BRL"}
	}`
	first := performWagerTransactionRequest(handler, requestBody, "provider-a:external-bet-1")
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d; body=%s", first.Code, first.Body)
	}
	firstResult := decodeWagerHTTPResult(t, first)
	if firstResult.Status != "PROCESSED" || firstResult.Balance.Amount() != "75.00" || firstResult.IdempotentReplay {
		t.Fatalf("first result = %#v, want processed non-replay with balance 75.00", firstResult)
	}

	replay := performWagerTransactionRequest(handler, requestBody, "provider-a:external-bet-1")
	if replay.Code != http.StatusOK {
		t.Fatalf("replay status = %d; body=%s", replay.Code, replay.Body)
	}
	replayResult := decodeWagerHTTPResult(t, replay)
	if replayResult.TransactionID != firstResult.TransactionID ||
		replayResult.Balance.Amount() != "75.00" || !replayResult.IdempotentReplay {
		t.Fatalf("replay result = %#v, want original transaction and balance", replayResult)
	}

	conflictingBody := bytes.ReplaceAll([]byte(requestBody), []byte(`"25.00"`), []byte(`"20.00"`))
	conflict := performWagerTransactionRequest(handler, string(conflictingBody), "provider-a:external-bet-1")
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, want %d; body=%s", conflict.Code, http.StatusConflict, conflict.Body)
	}

	var balanceInCents int64
	var ledgerEntries int
	if err := pool.QueryRow(ctx, `SELECT balance_in_cents FROM wallets WHERE id = 'wallet-1'`).Scan(&balanceInCents); err != nil {
		t.Fatalf("read wallet balance: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM wallet_ledger_entries`).Scan(&ledgerEntries); err != nil {
		t.Fatalf("count ledger entries: %v", err)
	}
	if balanceInCents != 7500 || ledgerEntries != 1 {
		t.Fatalf("financial state = balance %d ledger %d, want 7500/1", balanceInCents, ledgerEntries)
	}
}

type integrationSequenceIDGenerator struct{ next int }

func (generator *integrationSequenceIDGenerator) NewID() (string, error) {
	generator.next++
	return fmt.Sprintf("generated-%d", generator.next), nil
}

type integrationFixedClock struct{ now time.Time }

func (clock integrationFixedClock) Now() time.Time { return clock.now }

func performWagerTransactionRequest(
	handler http.Handler,
	body string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/wagering/transactions", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.Header.Set("X-Correlation-ID", "correlation-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type wagerHTTPResult struct {
	TransactionID    string       `json:"transactionId"`
	Status           string       `json:"status"`
	Balance          domain.Money `json:"balance"`
	IdempotentReplay bool         `json:"idempotentReplay"`
}

func decodeWagerHTTPResult(t *testing.T, response *httptest.ResponseRecorder) wagerHTTPResult {
	t.Helper()
	var result wagerHTTPResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode wager HTTP response: %v", err)
	}
	return result
}

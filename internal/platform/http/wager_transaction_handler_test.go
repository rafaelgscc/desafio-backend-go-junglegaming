package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
)

func TestWagerTransactionHandlerProcessesBet(t *testing.T) {
	now := time.Date(2026, time.October, 1, 19, 0, 0, 0, time.UTC)
	balance, err := domain.NewMoney("975.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	executor := &executeWagerTransactionStub{
		execute: func(
			_ context.Context,
			command application.ExecuteWagerTransactionCommand,
		) (application.ExecuteWagerTransactionResult, error) {
			return application.ExecuteWagerTransactionResult{
				TransactionID: command.TransactionID,
				Status:        domain.WagerTransactionStatusProcessed,
				Balance:       balance,
				HasBalance:    true,
			}, nil
		},
	}
	handler := newWagerTransactionHandlerForTest(t, executor, now)
	request := newWagerTransactionRequest(http.MethodPost, validWagerTransactionJSON())
	request.Header.Set("Idempotency-Key", "provider-a:transaction-123")
	request.Header.Set("X-Correlation-ID", "correlation-1")
	request.Header.Set("X-Request-ID", "request-1")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
	}
	var body struct {
		TransactionID    string       `json:"transactionId"`
		Status           string       `json:"status"`
		Balance          domain.Money `json:"balance"`
		IdempotentReplay bool         `json:"idempotentReplay"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TransactionID != "transaction-id" || body.Status != "PROCESSED" ||
		body.Balance.Amount() != "975.00" || body.IdempotentReplay {
		t.Fatalf("unexpected response: %#v", body)
	}
	command := executor.received
	if command.IdempotencyKey != "provider-a:transaction-123" ||
		command.Payload.ProviderID != "provider-a" ||
		command.Payload.Kind != domain.WagerTransactionKindBet ||
		command.Payload.Money.Amount() != "25.00" ||
		command.CorrelationID != "correlation-1" || command.CausationID != "request-1" ||
		!command.OccurredAt.Equal(now) ||
		!command.NextReferenceAttemptAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("unexpected application command: %#v", command)
	}
}

func TestWagerTransactionHandlerRequiresIdempotencyKey(t *testing.T) {
	executor := &executeWagerTransactionStub{}
	handler := newWagerTransactionHandlerForTest(t, executor, time.Now())
	request := newWagerTransactionRequest(http.MethodPost, validWagerTransactionJSON())
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, response, "IDEMPOTENCY_KEY_REQUIRED")
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}
}

func TestWagerTransactionHandlerRejectsProviderMismatch(t *testing.T) {
	executor := &executeWagerTransactionStub{}
	handler := newWagerTransactionHandlerForTest(t, executor, time.Now())
	request := newWagerTransactionRequest(http.MethodPost, validWagerTransactionJSON())
	request = request.WithContext(platformauth.ContextWithIdentity(
		request.Context(),
		platformauth.Identity{Subject: "service-account-provider-b", ProviderID: "provider-b"},
	))
	request.Header.Set("Idempotency-Key", "provider-a:transaction-123")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body)
	}
	assertErrorCode(t, response, "PROVIDER_MISMATCH")
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}
}

func TestWagerTransactionHandlerMapsResultsAndErrors(t *testing.T) {
	balance, err := domain.NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	testCases := []struct {
		name       string
		result     application.ExecuteWagerTransactionResult
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name: "pending reference", wantStatus: http.StatusAccepted,
			result: application.ExecuteWagerTransactionResult{
				TransactionID: "transaction-1", Status: domain.WagerTransactionStatusPendingReference,
				Balance: balance, HasBalance: true,
			},
		},
		{
			name: "business rejection", wantStatus: http.StatusUnprocessableEntity,
			result: application.ExecuteWagerTransactionResult{
				TransactionID: "transaction-1", Status: domain.WagerTransactionStatusRejected,
				Balance: balance, HasBalance: true,
				FailureCode: domain.WagerTransactionFailureCodeInsufficientFunds,
			},
		},
		{
			name: "idempotency conflict", err: application.ErrIdempotencyKeyConflict,
			wantStatus: http.StatusConflict, wantCode: "IDEMPOTENCY_KEY_CONFLICT",
		},
		{
			name: "external transaction conflict", err: application.ErrExternalTransactionConflict,
			wantStatus: http.StatusConflict, wantCode: "EXTERNAL_TRANSACTION_CONFLICT",
		},
		{
			name: "wallet not found", err: application.ErrWalletNotFound,
			wantStatus: http.StatusNotFound, wantCode: "WALLET_NOT_FOUND",
		},
		{
			name: "concurrent update", err: application.ErrConcurrentWalletUpdate,
			wantStatus: http.StatusServiceUnavailable, wantCode: "TEMPORARILY_UNAVAILABLE",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			executor := &executeWagerTransactionStub{
				execute: func(context.Context, application.ExecuteWagerTransactionCommand) (application.ExecuteWagerTransactionResult, error) {
					return testCase.result, testCase.err
				},
			}
			handler := newWagerTransactionHandlerForTest(t, executor, time.Now())
			request := newWagerTransactionRequest(http.MethodPost, validWagerTransactionJSON())
			request.Header.Set("Idempotency-Key", "provider-a:transaction-123")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, testCase.wantStatus, response.Body)
			}
			if testCase.wantCode != "" {
				assertErrorCode(t, response, testCase.wantCode)
			}
		})
	}
}

func TestWagerTransactionHandlerRejectsInvalidRequests(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{name: "unknown kind", body: `{
			"providerId":"provider-a","externalTransactionId":"transaction-123",
			"playerId":"player-1","walletId":"wallet-1","roundId":"round-1",
			"gameId":"game-1","kind":"OPENING",
			"money":{"amount":"25.00","currency":"BRL"}
		}`},
		{name: "unknown field", body: `{
			"providerId":"provider-a","externalTransactionId":"transaction-123",
			"playerId":"player-1","walletId":"wallet-1","roundId":"round-1",
			"gameId":"game-1","kind":"BET","unexpected":true,
			"money":{"amount":"25.00","currency":"BRL"}
		}`},
		{name: "refund without reference", body: `{
			"providerId":"provider-a","externalTransactionId":"transaction-123",
			"playerId":"player-1","walletId":"wallet-1","roundId":"round-1",
			"gameId":"game-1","kind":"REFUND",
			"money":{"amount":"25.00","currency":"BRL"}
		}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			executor := &executeWagerTransactionStub{
				execute: func(context.Context, application.ExecuteWagerTransactionCommand) (application.ExecuteWagerTransactionResult, error) {
					return application.ExecuteWagerTransactionResult{}, domain.ErrInvalidWagerTransactionKind
				},
			}
			handler := newWagerTransactionHandlerForTest(t, executor, time.Now())
			request := newWagerTransactionRequest(http.MethodPost, testCase.body)
			request.Header.Set("Idempotency-Key", "provider-a:transaction-123")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body)
			}
			assertErrorCode(t, response, "INVALID_REQUEST")
		})
	}
}

type executeWagerTransactionStub struct {
	execute  func(context.Context, application.ExecuteWagerTransactionCommand) (application.ExecuteWagerTransactionResult, error)
	received application.ExecuteWagerTransactionCommand
	calls    int
}

func (stub *executeWagerTransactionStub) Execute(
	ctx context.Context,
	command application.ExecuteWagerTransactionCommand,
) (application.ExecuteWagerTransactionResult, error) {
	stub.calls++
	stub.received = command
	if stub.execute == nil {
		return application.ExecuteWagerTransactionResult{}, errors.New("unexpected executor call")
	}
	return stub.execute(ctx, command)
}

func newWagerTransactionHandlerForTest(
	t *testing.T,
	executor ExecuteWagerTransactionExecutor,
	now time.Time,
) *WagerTransactionHandler {
	t.Helper()
	handler, err := NewWagerTransactionHandler(
		executor,
		&sequenceIDGenerator{ids: []string{
			"transaction-id", "ledger-id", "outcome-event-id", "balance-event-id", "correlation-id",
		}},
		fixedClock{now: now},
	)
	if err != nil {
		t.Fatalf("NewWagerTransactionHandler() unexpected error: %v", err)
	}
	return handler
}

func newWagerTransactionRequest(method, body string) *http.Request {
	request := httptest.NewRequest(method, "/wagering/transactions", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(platformauth.ContextWithIdentity(
		request.Context(),
		platformauth.Identity{Subject: "service-account-provider-a", ProviderID: "provider-a"},
	))
}

func validWagerTransactionJSON() string {
	return `{
		"providerId":"provider-a",
		"externalTransactionId":"transaction-123",
		"playerId":"player-1",
		"walletId":"wallet-1",
		"roundId":"round-987",
		"gameId":"fortune-chimp",
		"kind":"BET",
		"money":{"amount":"25.00","currency":"BRL"}
	}`
}

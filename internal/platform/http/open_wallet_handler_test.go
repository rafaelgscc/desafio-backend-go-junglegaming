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
)

func TestOpenWalletHandler(t *testing.T) {
	now := time.Date(2026, time.October, 1, 17, 0, 0, 0, time.UTC)

	t.Run("creates wallet", func(t *testing.T) {
		executor := &openWalletExecutorStub{}
		executor.execute = func(
			_ context.Context,
			command application.OpenWalletCommand,
		) (application.OpenWalletResult, error) {
			executor.received = command
			return application.OpenWalletResult{
				WalletID: command.WalletID,
				PlayerID: command.PlayerID,
				Balance:  command.InitialBalance,
				Version:  1,
			}, nil
		}

		handler, err := NewOpenWalletHandler(
			executor,
			&sequenceIDGenerator{ids: []string{
				"wallet-1",
				"opening-1",
				"ledger-1",
				"processed-event-1",
				"balance-event-1",
			}},
			fixedClock{now: now},
		)
		if err != nil {
			t.Fatalf("NewOpenWalletHandler() unexpected error: %v", err)
		}

		request := httptest.NewRequest(http.MethodPost, "/wallets", bytes.NewBufferString(`{
			"playerId":"player-1",
			"initialBalance":{"amount":"1000.00","currency":"BRL"}
		}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Correlation-ID", "correlation-1")
		request.Header.Set("X-Request-ID", "request-1")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body)
		}
		if got := response.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body struct {
			ID       string       `json:"id"`
			PlayerID string       `json:"playerId"`
			Balance  domain.Money `json:"balance"`
			Version  int64        `json:"version"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.ID != "wallet-1" || body.PlayerID != "player-1" ||
			body.Balance.Amount() != "1000.00" || body.Balance.Currency() != "BRL" ||
			body.Version != 1 {
			t.Fatalf("unexpected response: %+v", body)
		}

		command := executor.received
		if command.WalletID != "wallet-1" ||
			command.OpeningTransactionID != "opening-1" ||
			command.LedgerEntryID != "ledger-1" ||
			command.ProcessedEventID != "processed-event-1" ||
			command.BalanceChangedEventID != "balance-event-1" ||
			command.CorrelationID != "correlation-1" ||
			command.CausationID != "request-1" ||
			!command.CreatedAt.Equal(now) {
			t.Fatalf("unexpected application command: %+v", command)
		}
	})

	t.Run("rejects invalid money", func(t *testing.T) {
		executor := &openWalletExecutorStub{}
		handler, err := NewOpenWalletHandler(
			executor,
			&sequenceIDGenerator{},
			fixedClock{now: now},
		)
		if err != nil {
			t.Fatalf("NewOpenWalletHandler() unexpected error: %v", err)
		}

		request := httptest.NewRequest(http.MethodPost, "/wallets", bytes.NewBufferString(`{
			"playerId":"player-1",
			"initialBalance":{"amount":"10.999","currency":"BRL"}
		}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body)
		}
		if executor.calls != 0 {
			t.Fatalf("executor calls = %d, want 0", executor.calls)
		}
		assertErrorCode(t, response, "INVALID_REQUEST")
	})

	t.Run("returns conflict for duplicate player and currency", func(t *testing.T) {
		executor := &openWalletExecutorStub{
			execute: func(
				context.Context,
				application.OpenWalletCommand,
			) (application.OpenWalletResult, error) {
				return application.OpenWalletResult{}, application.ErrWalletAlreadyExists
			},
		}
		handler, err := NewOpenWalletHandler(
			executor,
			&sequenceIDGenerator{ids: []string{"wallet-1", "opening-1", "ledger-1", "event-1", "event-2", "correlation-1"}},
			fixedClock{now: now},
		)
		if err != nil {
			t.Fatalf("NewOpenWalletHandler() unexpected error: %v", err)
		}

		request := httptest.NewRequest(http.MethodPost, "/wallets", bytes.NewBufferString(`{
			"playerId":"player-1",
			"initialBalance":{"amount":"100.00","currency":"BRL"}
		}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body)
		}
		assertErrorCode(t, response, "WALLET_ALREADY_EXISTS")
	})
}

type openWalletExecutorStub struct {
	execute  func(context.Context, application.OpenWalletCommand) (application.OpenWalletResult, error)
	received application.OpenWalletCommand
	calls    int
}

func (stub *openWalletExecutorStub) Execute(
	ctx context.Context,
	command application.OpenWalletCommand,
) (application.OpenWalletResult, error) {
	stub.calls++
	stub.received = command
	if stub.execute == nil {
		return application.OpenWalletResult{}, errors.New("unexpected executor call")
	}
	return stub.execute(ctx, command)
}

type sequenceIDGenerator struct {
	ids   []string
	index int
}

func (generator *sequenceIDGenerator) NewID() (string, error) {
	if generator.index >= len(generator.ids) {
		return "", errors.New("no ID configured")
	}
	id := generator.ids[generator.index]
	generator.index++
	return id, nil
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != want {
		t.Fatalf("error code = %q, want %q", body.Error.Code, want)
	}
}

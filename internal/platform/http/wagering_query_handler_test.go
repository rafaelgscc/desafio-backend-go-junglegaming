package httpadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
)

type wageringQueryExecutorStub struct {
	wallet      domain.Wallet
	transaction domain.WagerTransaction
	page        application.WalletLedgerPage
	result      application.ReconciliationResult
	err         error
	providerID  string
}

func (stub *wageringQueryExecutorStub) GetWallet(context.Context, string) (domain.Wallet, error) {
	return stub.wallet, stub.err
}
func (stub *wageringQueryExecutorStub) ListWalletLedger(
	context.Context, string, *application.LedgerCursor, int,
) (application.WalletLedgerPage, error) {
	return stub.page, stub.err
}
func (stub *wageringQueryExecutorStub) GetWagerTransaction(
	_ context.Context, providerID, _ string,
) (domain.WagerTransaction, error) {
	stub.providerID = providerID
	return stub.transaction, stub.err
}
func (stub *wageringQueryExecutorStub) GetWagerTransactionByExternalID(
	_ context.Context, providerID, _ string,
) (domain.WagerTransaction, error) {
	stub.providerID = providerID
	return stub.transaction, stub.err
}
func (stub *wageringQueryExecutorStub) ReconcileWallet(
	context.Context, string,
) (application.ReconciliationResult, error) {
	return stub.result, stub.err
}

func TestWageringQueryHandlerReturnsWallet(t *testing.T) {
	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	balance, _ := domain.NewMoney("100.00", "BRL")
	wallet, _ := domain.NewWallet("wallet-1", "player-1", balance, now)
	handler, _ := NewWageringQueryHandler(&wageringQueryExecutorStub{wallet: wallet}, newTestMetrics())
	request := httptest.NewRequest(http.MethodGet, "/wallets/wallet-1", nil)
	request.SetPathValue("walletId", "wallet-1")
	response := httptest.NewRecorder()

	handler.GetWallet(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"amount":"100.00"`) {
		t.Fatalf("status/body = %d/%s", response.Code, response.Body.String())
	}
}

func TestWageringQueryHandlerRejectsProviderMismatch(t *testing.T) {
	stub := &wageringQueryExecutorStub{}
	handler, _ := NewWageringQueryHandler(stub, newTestMetrics())
	request := httptest.NewRequest(http.MethodGet, "/providers/provider-b/wagering/transactions/external-1", nil)
	request.SetPathValue("providerId", "provider-b")
	request.SetPathValue("externalTransactionId", "external-1")
	request = request.WithContext(platformauth.ContextWithIdentity(
		request.Context(), platformauth.Identity{Subject: "provider-a", ProviderID: "provider-a"},
	))
	response := httptest.NewRecorder()

	handler.GetWagerTransactionByExternalID(response, request)

	if response.Code != http.StatusForbidden || stub.providerID != "" {
		t.Fatalf("status/provider = %d/%q", response.Code, stub.providerID)
	}
}

func TestLedgerCursorRoundTrip(t *testing.T) {
	want := application.LedgerCursor{
		CreatedAt: time.Date(2026, time.October, 1, 12, 0, 0, 123, time.UTC),
		EntryID:   "entry-1",
	}
	got, err := decodeLedgerCursor(encodeLedgerCursor(want))
	if err != nil || got.EntryID != want.EntryID || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("decoded cursor = %#v, error = %v", got, err)
	}
	if _, err := decodeLedgerCursor("not-base64!"); err == nil {
		t.Fatal("decodeLedgerCursor() error = nil for invalid cursor")
	}
}

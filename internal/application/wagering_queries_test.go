package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

type wageringQueryRepositoryStub struct {
	wallet         domain.Wallet
	entries        []domain.WalletLedgerEntry
	transaction    domain.WagerTransaction
	reconciliation ReconciliationResult
	err            error
	requestedLimit int
	providerID     string
	transactionID  string
}

func (stub *wageringQueryRepositoryStub) FindWallet(context.Context, string) (domain.Wallet, error) {
	return stub.wallet, stub.err
}

func (stub *wageringQueryRepositoryStub) ListWalletLedger(
	_ context.Context, _ string, _ *LedgerCursor, limit int,
) ([]domain.WalletLedgerEntry, error) {
	stub.requestedLimit = limit
	return stub.entries, stub.err
}

func (stub *wageringQueryRepositoryStub) FindWagerTransactionForProvider(
	_ context.Context, providerID, transactionID string,
) (domain.WagerTransaction, error) {
	stub.providerID, stub.transactionID = providerID, transactionID
	return stub.transaction, stub.err
}

func (stub *wageringQueryRepositoryStub) FindWagerTransactionByExternalID(
	_ context.Context, providerID, transactionID string,
) (domain.WagerTransaction, error) {
	stub.providerID, stub.transactionID = providerID, transactionID
	return stub.transaction, stub.err
}

func (stub *wageringQueryRepositoryStub) ReconcileWallet(context.Context, string) (ReconciliationResult, error) {
	return stub.reconciliation, stub.err
}

func TestWageringQueryServicePaginatesLedgerWithStableCursor(t *testing.T) {
	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	wallet := mustQueryWallet(t, now)
	stub := &wageringQueryRepositoryStub{wallet: wallet, entries: []domain.WalletLedgerEntry{
		mustLedgerEntry(t, "entry-1", "tx-1", "10.00", "20.00", now),
		mustLedgerEntry(t, "entry-2", "tx-2", "20.00", "30.00", now),
		mustLedgerEntry(t, "entry-3", "tx-3", "30.00", "40.00", now),
	}}
	service, err := NewWageringQueryService(stub)
	if err != nil {
		t.Fatalf("NewWageringQueryService() error = %v", err)
	}

	page, err := service.ListWalletLedger(context.Background(), "wallet-1", nil, 2)
	if err != nil {
		t.Fatalf("ListWalletLedger() error = %v", err)
	}
	if stub.requestedLimit != 3 || len(page.Entries) != 2 || page.NextCursor == nil {
		t.Fatalf("page = %#v, requested limit = %d", page, stub.requestedLimit)
	}
	if page.NextCursor.EntryID != "entry-2" || !page.NextCursor.CreatedAt.Equal(now) {
		t.Fatalf("next cursor = %#v, want entry-2 at %v", page.NextCursor, now)
	}
}

func TestWageringQueryServiceScopesTransactionByProvider(t *testing.T) {
	stub := &wageringQueryRepositoryStub{}
	service, _ := NewWageringQueryService(stub)
	_, _ = service.GetWagerTransaction(context.Background(), "provider-a", "transaction-1")
	if stub.providerID != "provider-a" || stub.transactionID != "transaction-1" {
		t.Fatalf("repository scope = %q/%q", stub.providerID, stub.transactionID)
	}
}

func TestWageringQueryServiceRejectsInvalidQueries(t *testing.T) {
	service, _ := NewWageringQueryService(&wageringQueryRepositoryStub{})
	for _, test := range []func() error{
		func() error { _, err := service.GetWallet(context.Background(), " "); return err },
		func() error { _, err := service.ListWalletLedger(context.Background(), "wallet-1", nil, 0); return err },
		func() error {
			_, err := service.ListWalletLedger(context.Background(), "wallet-1", nil, MaxLedgerPageSize+1)
			return err
		},
		func() error { _, err := service.GetWagerTransaction(context.Background(), "", "tx-1"); return err },
	} {
		if err := test(); !errors.Is(err, ErrInvalidWageringQuery) {
			t.Fatalf("error = %v, want ErrInvalidWageringQuery", err)
		}
	}
}

func mustQueryWallet(t *testing.T, now time.Time) domain.Wallet {
	t.Helper()
	money, _ := domain.NewMoney("10.00", "BRL")
	wallet, err := domain.NewWallet("wallet-1", "player-1", money, now)
	if err != nil {
		t.Fatal(err)
	}
	return wallet
}

func mustLedgerEntry(
	t *testing.T, id, transactionID, beforeAmount, afterAmount string, now time.Time,
) domain.WalletLedgerEntry {
	t.Helper()
	money, _ := domain.NewMoney("10.00", "BRL")
	before, _ := domain.NewMoney(beforeAmount, "BRL")
	after, _ := domain.NewMoney(afterAmount, "BRL")
	entry, err := domain.NewWalletLedgerEntry(
		id, "wallet-1", transactionID, domain.LedgerDirectionCredit,
		money, before, after, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

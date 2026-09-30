package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestProcessBetDebitsWalletAndPersistsFinancialResult(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processBetFixtures(t, "100.00", "25.00")
	unitOfWork := &fakeWageringUnitOfWork{wallet: wallet, transaction: transaction}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessBetCommand{
		TransactionID: transaction.ID(),
		LedgerEntryID: "ledger-entry-1",
		ProcessedAt:   now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if result.Status != domain.WagerTransactionStatusProcessed {
		t.Errorf("result.Status = %q, want %q", result.Status, domain.WagerTransactionStatusProcessed)
	}
	if got := result.Balance.Amount(); got != "75.00" {
		t.Errorf("result.Balance = %q, want 75.00", got)
	}
	if result.WalletVersion != 2 {
		t.Errorf("result.WalletVersion = %d, want 2", result.WalletVersion)
	}

	if got := unitOfWork.wallet.Balance().Amount(); got != "75.00" {
		t.Errorf("persisted wallet balance = %q, want 75.00", got)
	}
	if unitOfWork.transaction.Status() != domain.WagerTransactionStatusProcessed {
		t.Errorf("persisted transaction status = %q, want PROCESSED", unitOfWork.transaction.Status())
	}
	if unitOfWork.ledgerEntry == nil {
		t.Fatal("no ledger entry was persisted")
	}
	if unitOfWork.ledgerEntry.Direction() != domain.LedgerDirectionDebit {
		t.Errorf("ledger direction = %q, want DEBIT", unitOfWork.ledgerEntry.Direction())
	}
	if got := unitOfWork.ledgerEntry.BalanceBefore().Amount(); got != "100.00" {
		t.Errorf("ledger balance before = %q, want 100.00", got)
	}
	if got := unitOfWork.ledgerEntry.BalanceAfter().Amount(); got != "75.00" {
		t.Errorf("ledger balance after = %q, want 75.00", got)
	}
}

func TestProcessBetRejectsInsufficientFundsWithoutChangingWalletOrLedger(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processBetFixtures(t, "20.00", "25.00")
	unitOfWork := &fakeWageringUnitOfWork{wallet: wallet, transaction: transaction}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessBetCommand{
		TransactionID: transaction.ID(),
		LedgerEntryID: "ledger-entry-1",
		ProcessedAt:   now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if result.Status != domain.WagerTransactionStatusRejected {
		t.Errorf("result.Status = %q, want REJECTED", result.Status)
	}
	if got := result.Balance.Amount(); got != "20.00" {
		t.Errorf("result.Balance = %q, want 20.00", got)
	}
	if unitOfWork.wallet.Version() != 1 || unitOfWork.wallet.Balance().Amount() != "20.00" {
		t.Errorf("wallet changed after rejection: version=%d balance=%s", unitOfWork.wallet.Version(), unitOfWork.wallet.Balance().Amount())
	}
	if unitOfWork.ledgerEntry != nil {
		t.Error("ledger entry was persisted for rejected bet")
	}
	if unitOfWork.transaction.Status() != domain.WagerTransactionStatusRejected {
		t.Errorf("transaction status = %q, want REJECTED", unitOfWork.transaction.Status())
	}
	if unitOfWork.transaction.FailureCode() != domain.WagerTransactionFailureCodeInsufficientFunds {
		t.Errorf("failure code = %q, want %q", unitOfWork.transaction.FailureCode(), domain.WagerTransactionFailureCodeInsufficientFunds)
	}
}

func TestProcessBetRejectsInvalidCommandBeforeOpeningTransaction(t *testing.T) {
	t.Parallel()

	unitOfWork := &fakeWageringUnitOfWork{}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	_, err = useCase.Execute(context.Background(), ProcessBetCommand{})
	if !errors.Is(err, ErrInvalidProcessBetCommand) {
		t.Fatalf("Execute() error = %v, want ErrInvalidProcessBetCommand", err)
	}
	if unitOfWork.calls != 0 {
		t.Errorf("WithinTransaction() calls = %d, want 0", unitOfWork.calls)
	}
}

func TestProcessBetRollsBackEveryChangeWhenPersistenceFails(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processBetFixtures(t, "100.00", "25.00")
	persistenceErr := errors.New("append ledger failed")
	unitOfWork := &fakeWageringUnitOfWork{
		wallet:          wallet,
		transaction:     transaction,
		appendLedgerErr: persistenceErr,
	}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	_, err = useCase.Execute(context.Background(), ProcessBetCommand{
		TransactionID: transaction.ID(),
		LedgerEntryID: "ledger-entry-1",
		ProcessedAt:   now.Add(time.Second),
	})
	if !errors.Is(err, persistenceErr) {
		t.Fatalf("Execute() error = %v, want persistence error", err)
	}
	if unitOfWork.wallet.Balance().Amount() != "100.00" || unitOfWork.wallet.Version() != 1 {
		t.Errorf("wallet was committed after rollback: balance=%s version=%d", unitOfWork.wallet.Balance().Amount(), unitOfWork.wallet.Version())
	}
	if unitOfWork.transaction.Status() != domain.WagerTransactionStatusPending {
		t.Errorf("transaction status was committed as %q, want PENDING", unitOfWork.transaction.Status())
	}
	if unitOfWork.ledgerEntry != nil {
		t.Error("ledger entry was committed after rollback")
	}
}

func processBetFixtures(
	t *testing.T,
	walletAmount string,
	betAmount string,
) (domain.Wallet, domain.WagerTransaction, time.Time) {
	t.Helper()

	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	walletMoney, err := domain.NewMoney(walletAmount, "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected wallet setup error: %v", err)
	}
	wallet, err := domain.NewWallet("wallet-1", "player-1", walletMoney, now)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}
	betMoney, err := domain.NewMoney(betAmount, "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected bet setup error: %v", err)
	}
	transaction, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID:                    "wager-transaction-1",
		ExternalTransactionID: "provider-transaction-1",
		ProviderID:            "provider-a",
		IdempotencyKey:        "provider-a:provider-transaction-1",
		PayloadHash:           "sha256:payload",
		WalletID:              wallet.ID(),
		PlayerID:              wallet.PlayerID(),
		RoundID:               "round-1",
		GameID:                "game-1",
		Kind:                  domain.WagerTransactionKindBet,
		Money:                 betMoney,
		OccurredAt:            now,
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}
	return wallet, transaction, now
}

type fakeWageringUnitOfWork struct {
	wallet          domain.Wallet
	transaction     domain.WagerTransaction
	ledgerEntry     *domain.WalletLedgerEntry
	calls           int
	fail            error
	appendLedgerErr error
}

func (unitOfWork *fakeWageringUnitOfWork) WithinTransaction(
	_ context.Context,
	fn func(WageringTransaction) error,
) error {
	unitOfWork.calls++
	if unitOfWork.fail != nil {
		return unitOfWork.fail
	}

	working := &fakeWageringTransaction{
		wallet:          unitOfWork.wallet,
		transaction:     unitOfWork.transaction,
		appendLedgerErr: unitOfWork.appendLedgerErr,
	}
	if err := fn(working); err != nil {
		return err
	}

	unitOfWork.wallet = working.wallet
	unitOfWork.transaction = working.transaction
	unitOfWork.ledgerEntry = working.ledgerEntry
	return nil
}

type fakeWageringTransaction struct {
	wallet          domain.Wallet
	transaction     domain.WagerTransaction
	ledgerEntry     *domain.WalletLedgerEntry
	appendLedgerErr error
}

func (tx *fakeWageringTransaction) FindWagerTransactionForUpdate(
	_ context.Context,
	_ string,
) (domain.WagerTransaction, error) {
	return tx.transaction, nil
}

func (tx *fakeWageringTransaction) FindWalletForUpdate(
	_ context.Context,
	_ string,
) (domain.Wallet, error) {
	return tx.wallet, nil
}

func (tx *fakeWageringTransaction) SaveWagerTransaction(
	_ context.Context,
	transaction domain.WagerTransaction,
) error {
	tx.transaction = transaction
	return nil
}

func (tx *fakeWageringTransaction) SaveWallet(_ context.Context, wallet domain.Wallet) error {
	tx.wallet = wallet
	return nil
}

func (tx *fakeWageringTransaction) AppendWalletLedgerEntry(
	_ context.Context,
	entry domain.WalletLedgerEntry,
) error {
	if tx.appendLedgerErr != nil {
		return tx.appendLedgerErr
	}
	tx.ledgerEntry = &entry
	return nil
}

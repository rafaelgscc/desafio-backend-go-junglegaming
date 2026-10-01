package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestOpenWalletWithPositiveBalancePersistsOpeningLedgerAndEvents(t *testing.T) {
	t.Parallel()

	command := validOpenWalletCommand(t, "1000.00")
	unitOfWork := &fakeWageringUnitOfWork{}
	useCase, err := NewOpenWalletUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewOpenWalletUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if !unitOfWork.insertedWallet || result.Balance.Amount() != "1000.00" || result.Version != 1 {
		t.Errorf("wallet result = %#v, inserted=%v", result, unitOfWork.insertedWallet)
	}
	if !unitOfWork.insertedTransaction ||
		unitOfWork.transaction.Kind() != domain.WagerTransactionKindOpening ||
		unitOfWork.transaction.Status() != domain.WagerTransactionStatusProcessed {
		t.Errorf("opening transaction = kind %q status %q inserted=%v", unitOfWork.transaction.Kind(), unitOfWork.transaction.Status(), unitOfWork.insertedTransaction)
	}
	if unitOfWork.ledgerEntry == nil ||
		unitOfWork.ledgerEntry.Direction() != domain.LedgerDirectionCredit ||
		unitOfWork.ledgerEntry.BalanceBefore().Amount() != "0.00" ||
		unitOfWork.ledgerEntry.BalanceAfter().Amount() != "1000.00" {
		t.Fatalf("opening ledger entry = %#v", unitOfWork.ledgerEntry)
	}
	if len(unitOfWork.outboxEvents) != 2 ||
		unitOfWork.outboxEvents[0].EventType != IntegrationEventTypeWagerTransactionProcessed ||
		unitOfWork.outboxEvents[1].EventType != IntegrationEventTypeWalletBalanceChanged {
		t.Errorf("opening outbox events = %#v", unitOfWork.outboxEvents)
	}
}

func TestOpenWalletWithZeroBalancePersistsOnlyWallet(t *testing.T) {
	t.Parallel()

	command := validOpenWalletCommand(t, "0.00")
	command.OpeningTransactionID = ""
	command.LedgerEntryID = ""
	command.ProcessedEventID = ""
	command.BalanceChangedEventID = ""
	command.CorrelationID = ""
	unitOfWork := &fakeWageringUnitOfWork{}
	useCase, err := NewOpenWalletUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewOpenWalletUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if !unitOfWork.insertedWallet || result.Balance.Amount() != "0.00" || result.Version != 1 {
		t.Errorf("zero wallet result = %#v, inserted=%v", result, unitOfWork.insertedWallet)
	}
	if unitOfWork.insertedTransaction || unitOfWork.ledgerEntry != nil || len(unitOfWork.outboxEvents) != 0 {
		t.Errorf("zero opening created financial artifacts: transaction=%v ledger=%v events=%d", unitOfWork.insertedTransaction, unitOfWork.ledgerEntry != nil, len(unitOfWork.outboxEvents))
	}
}

func TestOpenWalletRejectsDuplicatePlayerAndCurrency(t *testing.T) {
	t.Parallel()

	command := validOpenWalletCommand(t, "1000.00")
	unitOfWork := &fakeWageringUnitOfWork{walletExists: true}
	useCase, err := NewOpenWalletUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewOpenWalletUseCase() unexpected error: %v", err)
	}

	_, err = useCase.Execute(context.Background(), command)
	if !errors.Is(err, ErrWalletAlreadyExists) {
		t.Fatalf("Execute() error = %v, want ErrWalletAlreadyExists", err)
	}
	if unitOfWork.insertedWallet || unitOfWork.insertedTransaction {
		t.Error("wallet or opening transaction was inserted after duplicate detection")
	}
}

func TestOpenWalletRollsBackEverythingWhenOutboxFails(t *testing.T) {
	t.Parallel()

	command := validOpenWalletCommand(t, "1000.00")
	outboxErr := errors.New("append opening outbox failed")
	unitOfWork := &fakeWageringUnitOfWork{appendOutboxErr: outboxErr}
	useCase, err := NewOpenWalletUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewOpenWalletUseCase() unexpected error: %v", err)
	}

	_, err = useCase.Execute(context.Background(), command)
	if !errors.Is(err, outboxErr) {
		t.Fatalf("Execute() error = %v, want outbox error", err)
	}
	if unitOfWork.insertedWallet || unitOfWork.insertedTransaction ||
		unitOfWork.ledgerEntry != nil || len(unitOfWork.outboxEvents) != 0 {
		t.Error("opening artifacts were committed after rollback")
	}
}

func validOpenWalletCommand(t *testing.T, amount string) OpenWalletCommand {
	t.Helper()
	money, err := domain.NewMoney(amount, "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	return OpenWalletCommand{
		WalletID: "wallet-open-1", PlayerID: "player-open-1", InitialBalance: money,
		OpeningTransactionID: "opening-transaction-1", LedgerEntryID: "opening-ledger-1",
		ProcessedEventID: "opening-processed-event-1", BalanceChangedEventID: "opening-balance-event-1",
		CorrelationID: "opening-correlation-1", CausationID: "opening-request-1",
		CreatedAt: time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	}
}

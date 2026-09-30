package application

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestProcessWinCreditsWalletAndPersistsLedgerAndEvents(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processWagerFixtures(
		t,
		domain.WagerTransactionKindWin,
		"100.00",
		"25.00",
	)
	unitOfWork := &fakeWageringUnitOfWork{wallet: wallet, transaction: transaction}
	useCase, err := NewProcessWinUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessWinUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessWinCommand{
		TransactionID:         transaction.ID(),
		LedgerEntryID:         "ledger-entry-win-1",
		OutcomeEventID:        "event-win-processed-1",
		BalanceChangedEventID: "event-win-balance-1",
		CorrelationID:         "correlation-win-1",
		CausationID:           "request-win-1",
		ProcessedAt:           now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if result.Status != domain.WagerTransactionStatusProcessed ||
		result.Balance.Amount() != "125.00" || result.WalletVersion != 2 {
		t.Errorf("result = status %q balance %s version %d, want PROCESSED 125.00 2", result.Status, result.Balance.Amount(), result.WalletVersion)
	}
	if unitOfWork.ledgerEntry == nil || unitOfWork.ledgerEntry.Direction() != domain.LedgerDirectionCredit {
		t.Fatalf("ledger entry = %#v, want CREDIT", unitOfWork.ledgerEntry)
	}
	if unitOfWork.ledgerEntry.BalanceBefore().Amount() != "100.00" ||
		unitOfWork.ledgerEntry.BalanceAfter().Amount() != "125.00" {
		t.Errorf("ledger balances = %s -> %s, want 100.00 -> 125.00", unitOfWork.ledgerEntry.BalanceBefore().Amount(), unitOfWork.ledgerEntry.BalanceAfter().Amount())
	}
	if len(unitOfWork.outboxEvents) != 2 ||
		unitOfWork.outboxEvents[0].EventType != IntegrationEventTypeWagerTransactionProcessed ||
		unitOfWork.outboxEvents[1].EventType != IntegrationEventTypeWalletBalanceChanged {
		t.Errorf("outbox events = %#v, want processed and balance changed", unitOfWork.outboxEvents)
	}
}

func TestProcessLossCompletesWithoutChangingWalletOrCreatingLedger(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processWagerFixtures(
		t,
		domain.WagerTransactionKindLoss,
		"100.00",
		"0.00",
	)
	unitOfWork := &fakeWageringUnitOfWork{wallet: wallet, transaction: transaction}
	useCase, err := NewProcessLossUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessLossUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessLossCommand{
		TransactionID:  transaction.ID(),
		OutcomeEventID: "event-loss-processed-1",
		CorrelationID:  "correlation-loss-1",
		CausationID:    "request-loss-1",
		ProcessedAt:    now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if result.Status != domain.WagerTransactionStatusProcessed ||
		result.Balance.Amount() != "100.00" || result.WalletVersion != 1 {
		t.Errorf("result = status %q balance %s version %d, want PROCESSED 100.00 1", result.Status, result.Balance.Amount(), result.WalletVersion)
	}
	if unitOfWork.wallet.Balance().Amount() != "100.00" || unitOfWork.wallet.Version() != 1 {
		t.Errorf("wallet changed for LOSS: balance=%s version=%d", unitOfWork.wallet.Balance().Amount(), unitOfWork.wallet.Version())
	}
	if unitOfWork.ledgerEntry != nil {
		t.Error("ledger entry was created for LOSS")
	}
	if len(unitOfWork.outboxEvents) != 1 ||
		unitOfWork.outboxEvents[0].EventType != IntegrationEventTypeWagerTransactionProcessed {
		t.Errorf("outbox events = %#v, want one processed event", unitOfWork.outboxEvents)
	}
}

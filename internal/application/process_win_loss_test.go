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

func TestProcessWinResolvesOptionalBetReference(t *testing.T) {
	t.Parallel()

	wallet, referencedBet, now := processWagerFixtures(
		t,
		domain.WagerTransactionKindBet,
		"100.00",
		"25.00",
	)
	processedBalance, err := domain.NewMoney("75.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	if err := referencedBet.MarkProcessed(processedBalance, now.Add(time.Second)); err != nil {
		t.Fatalf("MarkProcessed() BET setup error: %v", err)
	}
	winMoney, err := domain.NewMoney("50.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	win, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID: "win-1", ExternalTransactionID: "external-win-1",
		ProviderID: referencedBet.ProviderID(), IdempotencyKey: "key-win-1",
		PayloadHash: "hash-win-1", WalletID: referencedBet.WalletID(),
		PlayerID: referencedBet.PlayerID(), RoundID: referencedBet.RoundID(),
		GameID: referencedBet.GameID(), Kind: domain.WagerTransactionKindWin,
		Money: winMoney, ReferenceExternalTransactionID: referencedBet.ExternalTransactionID(),
		OccurredAt: now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() WIN setup error: %v", err)
	}
	unitOfWork := &fakeWageringUnitOfWork{
		wallet: wallet, transaction: win, referencedTransaction: referencedBet,
	}
	useCase, err := NewProcessWinUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessWinUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessWinCommand{
		TransactionID: win.ID(), LedgerEntryID: "ledger-win-1",
		OutcomeEventID: "outcome-win-1", BalanceChangedEventID: "balance-win-1",
		CorrelationID: "correlation-1", ProcessedAt: now.Add(3 * time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Status != domain.WagerTransactionStatusProcessed ||
		unitOfWork.transaction.ReferenceTransactionID() != referencedBet.ID() {
		t.Fatalf(
			"WIN status/reference = %q/%q, want PROCESSED/%q",
			result.Status,
			unitOfWork.transaction.ReferenceTransactionID(),
			referencedBet.ID(),
		)
	}
}

func TestProcessWinRejectsMissingOptionalReference(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processWagerFixtures(
		t,
		domain.WagerTransactionKindWin,
		"100.00",
		"25.00",
	)
	params := domain.NewExternalWagerTransactionParams{
		ID: transaction.ID(), ExternalTransactionID: transaction.ExternalTransactionID(),
		ProviderID: transaction.ProviderID(), IdempotencyKey: transaction.IdempotencyKey(),
		PayloadHash: transaction.PayloadHash(), WalletID: transaction.WalletID(),
		PlayerID: transaction.PlayerID(), RoundID: transaction.RoundID(), GameID: transaction.GameID(),
		Kind: domain.WagerTransactionKindWin, Money: transaction.Money(),
		ReferenceExternalTransactionID: "missing-bet", OccurredAt: now,
	}
	transaction, err := domain.NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() WIN setup error: %v", err)
	}
	unitOfWork := &fakeWageringUnitOfWork{
		wallet: wallet, transaction: transaction, referenceErr: ErrWagerTransactionNotFound,
	}
	useCase, err := NewProcessWinUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessWinUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessWinCommand{
		TransactionID: transaction.ID(), LedgerEntryID: "ledger-win-1",
		OutcomeEventID: "outcome-win-1", BalanceChangedEventID: "balance-win-1",
		CorrelationID: "correlation-1", ProcessedAt: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Status != domain.WagerTransactionStatusRejected ||
		result.FailureCode != domain.WagerTransactionFailureCodeReferenceNotFound ||
		unitOfWork.ledgerEntry != nil {
		t.Fatalf("result = %#v, want rejected missing reference without ledger", result)
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

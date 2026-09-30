package application

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestProcessRefundCreditsReferencedBet(t *testing.T) {
	t.Parallel()

	unitOfWork, command := reversalFixtures(
		t,
		domain.WagerTransactionKindRefund,
		domain.WagerTransactionKindBet,
	)
	useCase, err := NewProcessRefundUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessRefundUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	assertProcessedReversal(t, unitOfWork, result, domain.LedgerDirectionCredit, "100.00")
}

func TestProcessRollbackDebitsReferencedWin(t *testing.T) {
	t.Parallel()

	unitOfWork, command := reversalFixtures(
		t,
		domain.WagerTransactionKindRollback,
		domain.WagerTransactionKindWin,
	)
	useCase, err := NewProcessRollbackUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessRollbackUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	assertProcessedReversal(t, unitOfWork, result, domain.LedgerDirectionDebit, "100.00")
}

func TestProcessReversalPersistsPendingReferenceWhenReferenceIsMissing(t *testing.T) {
	t.Parallel()

	unitOfWork, command := reversalFixtures(
		t,
		domain.WagerTransactionKindRefund,
		domain.WagerTransactionKindBet,
	)
	unitOfWork.referenceErr = ErrWagerTransactionNotFound
	initialBalance := unitOfWork.wallet.Balance().Amount()
	initialVersion := unitOfWork.wallet.Version()
	useCase, err := NewProcessRefundUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessRefundUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Status != domain.WagerTransactionStatusPendingReference {
		t.Errorf("result.Status = %q, want PENDING_REFERENCE", result.Status)
	}
	if unitOfWork.wallet.Balance().Amount() != initialBalance || unitOfWork.wallet.Version() != initialVersion {
		t.Error("wallet changed while reference was pending")
	}
	if unitOfWork.ledgerEntry != nil {
		t.Error("ledger entry was created while reference was pending")
	}
	if len(unitOfWork.outboxEvents) != 1 ||
		unitOfWork.outboxEvents[0].EventType != IntegrationEventTypeWagerTransactionPendingReference {
		t.Fatalf("outbox events = %#v, want one pending-reference event", unitOfWork.outboxEvents)
	}
	data, ok := unitOfWork.outboxEvents[0].Data.(WagerTransactionPendingReferenceData)
	if !ok || data.ReferenceAttempts != 1 || !data.NextReferenceAttemptAt.Equal(command.NextReferenceAttemptAt) {
		t.Errorf("pending-reference event data = %#v", unitOfWork.outboxEvents[0].Data)
	}
}

func TestProcessReversalRejectsDuplicateWithoutMovingWallet(t *testing.T) {
	t.Parallel()

	unitOfWork, command := reversalFixtures(
		t,
		domain.WagerTransactionKindRefund,
		domain.WagerTransactionKindBet,
	)
	unitOfWork.hasProcessedReversal = true
	initialBalance := unitOfWork.wallet.Balance().Amount()
	useCase, err := NewProcessRefundUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessRefundUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Status != domain.WagerTransactionStatusRejected ||
		unitOfWork.transaction.FailureCode() != domain.WagerTransactionFailureCodeDuplicateReversal {
		t.Errorf("reversal = status %q code %q, want REJECTED DUPLICATE_REVERSAL", result.Status, unitOfWork.transaction.FailureCode())
	}
	if unitOfWork.wallet.Balance().Amount() != initialBalance || unitOfWork.ledgerEntry != nil {
		t.Error("duplicate reversal changed wallet or ledger")
	}
}

func TestProcessRollbackUsesDistinctInsufficientFundsFailureCode(t *testing.T) {
	t.Parallel()

	unitOfWork, command := reversalFixtures(
		t,
		domain.WagerTransactionKindRollback,
		domain.WagerTransactionKindWin,
	)
	lowBalance, err := domain.NewMoney("10.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	unitOfWork.wallet, err = domain.NewWallet(
		unitOfWork.wallet.ID(),
		unitOfWork.wallet.PlayerID(),
		lowBalance,
		unitOfWork.wallet.CreatedAt(),
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}
	useCase, err := NewProcessRollbackUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessRollbackUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Status != domain.WagerTransactionStatusRejected ||
		unitOfWork.transaction.FailureCode() != domain.WagerTransactionFailureCodeReversalInsufficientFunds {
		t.Errorf("rollback = status %q code %q, want REJECTED REVERSAL_INSUFFICIENT_FUNDS", result.Status, unitOfWork.transaction.FailureCode())
	}
	if unitOfWork.ledgerEntry != nil || unitOfWork.wallet.Balance().Amount() != "10.00" {
		t.Error("rejected rollback changed wallet or created ledger")
	}
}

func assertProcessedReversal(
	t *testing.T,
	unitOfWork *fakeWageringUnitOfWork,
	result ProcessReversalResult,
	direction domain.LedgerDirection,
	wantBalance string,
) {
	t.Helper()
	if result.Status != domain.WagerTransactionStatusProcessed || result.Balance.Amount() != wantBalance {
		t.Errorf("result = status %q balance %s, want PROCESSED %s", result.Status, result.Balance.Amount(), wantBalance)
	}
	if unitOfWork.transaction.ReferenceTransactionID() != unitOfWork.referencedTransaction.ID() {
		t.Errorf("resolved reference = %q, want %q", unitOfWork.transaction.ReferenceTransactionID(), unitOfWork.referencedTransaction.ID())
	}
	if unitOfWork.ledgerEntry == nil || unitOfWork.ledgerEntry.Direction() != direction {
		t.Fatalf("ledger entry = %#v, want direction %q", unitOfWork.ledgerEntry, direction)
	}
	if len(unitOfWork.outboxEvents) != 2 {
		t.Errorf("outbox event count = %d, want 2", len(unitOfWork.outboxEvents))
	}
}

func reversalFixtures(
	t *testing.T,
	reversalKind domain.WagerTransactionKind,
	referenceKind domain.WagerTransactionKind,
) (*fakeWageringUnitOfWork, ProcessReversalCommand) {
	t.Helper()

	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	initialMoney, err := domain.NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	wallet, err := domain.NewWallet("wallet-1", "player-1", initialMoney, now)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}
	amount, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	reference, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID: "reference-internal-1", ExternalTransactionID: "reference-external-1",
		ProviderID: "provider-a", IdempotencyKey: "provider-a:reference-external-1",
		PayloadHash: "sha256:reference", WalletID: wallet.ID(), PlayerID: wallet.PlayerID(),
		RoundID: "round-1", GameID: "game-1", Kind: referenceKind, Money: amount, OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction(reference) unexpected error: %v", err)
	}
	financialAt := now.Add(time.Second)
	if referenceKind == domain.WagerTransactionKindBet {
		err = wallet.Debit(amount, financialAt)
	} else {
		err = wallet.Credit(amount, financialAt)
	}
	if err != nil {
		t.Fatalf("wallet reference movement unexpected error: %v", err)
	}
	if err := reference.MarkProcessed(wallet.Balance(), financialAt); err != nil {
		t.Fatalf("reference.MarkProcessed() unexpected error: %v", err)
	}

	reversal, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID: "reversal-internal-1", ExternalTransactionID: "reversal-external-1",
		ProviderID: "provider-a", IdempotencyKey: "provider-a:reversal-external-1",
		PayloadHash: "sha256:reversal", WalletID: wallet.ID(), PlayerID: wallet.PlayerID(),
		RoundID: "round-1", GameID: "game-1", Kind: reversalKind, Money: amount,
		ReferenceExternalTransactionID: reference.ExternalTransactionID(),
		OccurredAt:                     now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction(reversal) unexpected error: %v", err)
	}
	processedAt := now.Add(3 * time.Second)
	return &fakeWageringUnitOfWork{
		wallet: wallet, transaction: reversal, referencedTransaction: reference,
	}, ProcessReversalCommand{
		TransactionID: reversal.ID(), LedgerEntryID: "ledger-reversal-1",
		OutcomeEventID: "event-reversal-outcome-1", BalanceChangedEventID: "event-reversal-balance-1",
		CorrelationID: "correlation-reversal-1", CausationID: "request-reversal-1",
		ProcessedAt: processedAt, NextReferenceAttemptAt: processedAt.Add(time.Minute),
	}
}

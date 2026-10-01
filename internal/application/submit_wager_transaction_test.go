package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestSubmitWagerTransactionPersistsNewPendingTransaction(t *testing.T) {
	t.Parallel()

	command := validSubmitWagerTransactionCommand(t)
	unitOfWork := &fakeWageringUnitOfWork{
		idempotencyErr: ErrWagerTransactionNotFound,
		referenceErr:   ErrWagerTransactionNotFound,
	}
	useCase, err := NewSubmitWagerTransactionUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewSubmitWagerTransactionUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if !unitOfWork.insertedTransaction {
		t.Fatal("new wager transaction was not inserted")
	}
	if result.TransactionID != command.TransactionID ||
		result.Status != domain.WagerTransactionStatusPending ||
		result.IdempotentReplay || result.HasBalance {
		t.Errorf("result = %#v, want new PENDING transaction", result)
	}
	if unitOfWork.transaction.PayloadHash() != command.PayloadHash {
		t.Errorf("persisted payload hash = %q, want %q", unitOfWork.transaction.PayloadHash(), command.PayloadHash)
	}
}

func TestSubmitWagerTransactionReturnsPersistedIdempotentReplay(t *testing.T) {
	t.Parallel()

	command := validSubmitWagerTransactionCommand(t)
	existing := wagerTransactionFromSubmitCommand(t, command)
	balance, err := domain.NewMoney("75.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	if err := existing.MarkProcessed(balance, command.OccurredAt.Add(time.Second)); err != nil {
		t.Fatalf("MarkProcessed() unexpected setup error: %v", err)
	}
	unitOfWork := &fakeWageringUnitOfWork{idempotentTransaction: existing}
	useCase, err := NewSubmitWagerTransactionUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewSubmitWagerTransactionUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), command)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if !result.IdempotentReplay || result.Status != domain.WagerTransactionStatusProcessed ||
		!result.HasBalance || result.Balance.Amount() != "75.00" {
		t.Errorf("result = %#v, want persisted processed replay with balance 75.00", result)
	}
	if unitOfWork.insertedTransaction {
		t.Error("transaction was inserted during idempotent replay")
	}
}

func TestSubmitWagerTransactionRejectsIdempotencyKeyWithDifferentPayload(t *testing.T) {
	t.Parallel()

	command := validSubmitWagerTransactionCommand(t)
	existing := wagerTransactionFromSubmitCommand(t, command)
	unitOfWork := &fakeWageringUnitOfWork{idempotentTransaction: existing}
	useCase, err := NewSubmitWagerTransactionUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewSubmitWagerTransactionUseCase() unexpected error: %v", err)
	}
	command.PayloadHash = "sha256:different-payload"

	_, err = useCase.Execute(context.Background(), command)
	if !errors.Is(err, ErrIdempotencyKeyConflict) {
		t.Fatalf("Execute() error = %v, want ErrIdempotencyKeyConflict", err)
	}
	if unitOfWork.insertedTransaction {
		t.Error("transaction was inserted after idempotency conflict")
	}
}

func TestSubmitWagerTransactionRejectsExternalTransactionWithAnotherKey(t *testing.T) {
	t.Parallel()

	command := validSubmitWagerTransactionCommand(t)
	existing := wagerTransactionFromSubmitCommand(t, command)
	unitOfWork := &fakeWageringUnitOfWork{
		idempotencyErr:        ErrWagerTransactionNotFound,
		referencedTransaction: existing,
	}
	useCase, err := NewSubmitWagerTransactionUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewSubmitWagerTransactionUseCase() unexpected error: %v", err)
	}
	command.IdempotencyKey = "provider-a:another-key"

	_, err = useCase.Execute(context.Background(), command)
	if !errors.Is(err, ErrExternalTransactionConflict) {
		t.Fatalf("Execute() error = %v, want ErrExternalTransactionConflict", err)
	}
	if unitOfWork.insertedTransaction {
		t.Error("transaction was inserted after external transaction conflict")
	}
}

func validSubmitWagerTransactionCommand(t *testing.T) SubmitWagerTransactionCommand {
	t.Helper()
	money, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	return SubmitWagerTransactionCommand{
		TransactionID: "wager-internal-1", ExternalTransactionID: "wager-external-1",
		ProviderID: "provider-a", IdempotencyKey: "provider-a:wager-external-1",
		PayloadHash: "sha256:payload", WalletID: "wallet-1", PlayerID: "player-1",
		RoundID: "round-1", GameID: "game-1", Kind: domain.WagerTransactionKindBet,
		Money: money, OccurredAt: time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	}
}

func wagerTransactionFromSubmitCommand(
	t *testing.T,
	command SubmitWagerTransactionCommand,
) domain.WagerTransaction {
	t.Helper()
	transaction, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID: command.TransactionID, ExternalTransactionID: command.ExternalTransactionID,
		ProviderID: command.ProviderID, IdempotencyKey: command.IdempotencyKey,
		PayloadHash: command.PayloadHash, WalletID: command.WalletID, PlayerID: command.PlayerID,
		RoundID: command.RoundID, GameID: command.GameID, Kind: command.Kind,
		Money: command.Money, ReferenceExternalTransactionID: command.ReferenceExternalTransactionID,
		OccurredAt: command.OccurredAt,
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}
	return transaction
}

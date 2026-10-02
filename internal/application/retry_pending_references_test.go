package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

type pendingReferenceRepositoryStub struct {
	items       []PendingReference
	claimErr    error
	released    []string
	failed      []string
	nextAttempt time.Time
}

func (stub *pendingReferenceRepositoryStub) ClaimPendingReferences(
	context.Context, string, time.Time, time.Time, int,
) ([]PendingReference, error) {
	return stub.items, stub.claimErr
}

func (stub *pendingReferenceRepositoryStub) ReleasePendingReference(
	_ context.Context, transactionID string, _ string,
) error {
	stub.released = append(stub.released, transactionID)
	return nil
}

func (stub *pendingReferenceRepositoryStub) FailPendingReference(
	_ context.Context, transactionID string, _ string, nextAttemptAt time.Time, _ string,
) error {
	stub.failed = append(stub.failed, transactionID)
	stub.nextAttempt = nextAttemptAt
	return nil
}

type reversalProcessorStub struct {
	result  ProcessReversalResult
	err     error
	command ProcessReversalCommand
	calls   int
}

func (stub *reversalProcessorStub) Execute(
	_ context.Context, command ProcessReversalCommand,
) (ProcessReversalResult, error) {
	stub.calls++
	stub.command = command
	return stub.result, stub.err
}

type pendingReferenceExpirerStub struct {
	result  ProcessReversalResult
	err     error
	command ExpirePendingReferenceCommand
	calls   int
}

func (stub *pendingReferenceExpirerStub) Execute(
	_ context.Context, command ExpirePendingReferenceCommand,
) (ProcessReversalResult, error) {
	stub.calls++
	stub.command = command
	return stub.result, stub.err
}

func TestRetryPendingReferencesReschedulesMissingReference(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	repository := &pendingReferenceRepositoryStub{items: []PendingReference{{
		TransactionID: "refund-1", Kind: domain.WagerTransactionKindRefund,
		ReferenceAttempts: 1,
	}}}
	refund := &reversalProcessorStub{result: ProcessReversalResult{
		Status: domain.WagerTransactionStatusPendingReference,
	}}
	rollback := &reversalProcessorStub{}
	expirer := &pendingReferenceExpirerStub{}
	useCase, err := NewRetryPendingReferencesUseCase(repository, refund, rollback, expirer)
	if err != nil {
		t.Fatal(err)
	}

	result, err := useCase.Execute(context.Background(), RetryPendingReferencesCommand{
		WorkerID: "reference-1", Now: now, LeaseDuration: time.Minute,
		RetryBaseDelay: time.Minute, MaxAttempts: 5, BatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 1 || result.Rescheduled != 1 || result.Failed != 0 {
		t.Fatalf("result = %#v", result)
	}
	if refund.calls != 1 || rollback.calls != 0 || expirer.calls != 0 {
		t.Fatalf("calls: refund=%d rollback=%d expirer=%d", refund.calls, rollback.calls, expirer.calls)
	}
	if refund.command.LedgerEntryID == "" || refund.command.OutcomeEventID == "" ||
		refund.command.BalanceChangedEventID == "" {
		t.Fatalf("retry IDs were not generated: %#v", refund.command)
	}
	if want := now.Add(2 * time.Minute); !refund.command.NextReferenceAttemptAt.Equal(want) {
		t.Fatalf("next attempt = %s, want %s", refund.command.NextReferenceAttemptAt, want)
	}
	if len(repository.released) != 1 || repository.released[0] != "refund-1" {
		t.Fatalf("released = %v", repository.released)
	}
}

func TestRetryPendingReferencesExpiresAtAttemptLimit(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	repository := &pendingReferenceRepositoryStub{items: []PendingReference{{
		TransactionID: "rollback-1", Kind: domain.WagerTransactionKindRollback,
		ReferenceAttempts: 5,
	}}}
	refund, rollback := &reversalProcessorStub{}, &reversalProcessorStub{}
	expirer := &pendingReferenceExpirerStub{result: ProcessReversalResult{
		Status: domain.WagerTransactionStatusRejected,
	}}
	useCase, _ := NewRetryPendingReferencesUseCase(repository, refund, rollback, expirer)

	result, err := useCase.Execute(context.Background(), RetryPendingReferencesCommand{
		WorkerID: "reference-1", Now: now, LeaseDuration: time.Minute,
		RetryBaseDelay: time.Minute, MaxAttempts: 5, BatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rejected != 1 || expirer.calls != 1 || refund.calls != 0 || rollback.calls != 0 {
		t.Fatalf("result=%#v calls: refund=%d rollback=%d expirer=%d", result, refund.calls, rollback.calls, expirer.calls)
	}
	if expirer.command.TransactionID != "rollback-1" || expirer.command.OutcomeEventID == "" {
		t.Fatalf("expire command = %#v", expirer.command)
	}
}

func TestRetryPendingReferencesSchedulesInfrastructureFailure(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	repository := &pendingReferenceRepositoryStub{items: []PendingReference{{
		TransactionID: "refund-1", Kind: domain.WagerTransactionKindRefund,
		ReferenceAttempts: 2,
	}}}
	processingError := errors.New("database unavailable")
	refund := &reversalProcessorStub{err: processingError}
	useCase, _ := NewRetryPendingReferencesUseCase(
		repository, refund, &reversalProcessorStub{}, &pendingReferenceExpirerStub{},
	)

	result, err := useCase.Execute(context.Background(), RetryPendingReferencesCommand{
		WorkerID: "reference-1", Now: now, LeaseDuration: time.Minute,
		RetryBaseDelay: time.Minute, MaxAttempts: 5, BatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != 1 || len(repository.failed) != 1 || len(repository.released) != 0 {
		t.Fatalf("result=%#v failed=%v released=%v", result, repository.failed, repository.released)
	}
	if want := now.Add(4 * time.Minute); !repository.nextAttempt.Equal(want) {
		t.Fatalf("next attempt = %s, want %s", repository.nextAttempt, want)
	}
}

func TestExpirePendingReferenceRejectsAndEmitsEvent(t *testing.T) {
	createdAt := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	money, _ := domain.NewMoney("30.00", "BRL")
	transaction, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID: "refund-1", ExternalTransactionID: "external-refund-1",
		ProviderID: "provider-1", IdempotencyKey: "key-1", PayloadHash: "hash-1",
		WalletID: "wallet-1", PlayerID: "player-1", RoundID: "round-1",
		GameID: "game-1", Kind: domain.WagerTransactionKindRefund, Money: money,
		ReferenceExternalTransactionID: "missing-bet", OccurredAt: createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.MarkPendingReference(createdAt.Add(time.Minute), createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	unitOfWork := &fakeWageringUnitOfWork{transaction: transaction}
	useCase, _ := NewExpirePendingReferenceUseCase(unitOfWork)

	result, err := useCase.Execute(context.Background(), ExpirePendingReferenceCommand{
		TransactionID: "refund-1", OutcomeEventID: "event-1",
		CorrelationID: "correlation-1", CausationID: "message-1",
		ExpiredAt: createdAt.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.WagerTransactionStatusRejected ||
		result.FailureCode != domain.WagerTransactionFailureCodeReferenceNotFound {
		t.Fatalf("result = %#v", result)
	}
	if len(unitOfWork.outboxEvents) != 1 ||
		unitOfWork.outboxEvents[0].EventType != IntegrationEventTypeWagerTransactionRejected {
		t.Fatalf("outbox events = %#v", unitOfWork.outboxEvents)
	}
}

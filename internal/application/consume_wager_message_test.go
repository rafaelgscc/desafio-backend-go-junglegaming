package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

type inboxRepositoryStub struct {
	claimStatus InboxClaimStatus
	claimErr    error
	completed   bool
	failed      bool
	message     InboxMessage
}

func (stub *inboxRepositoryStub) Claim(
	_ context.Context, message InboxMessage, _ string, _ time.Time,
) (InboxClaimStatus, error) {
	stub.message = message
	return stub.claimStatus, stub.claimErr
}
func (stub *inboxRepositoryStub) Complete(
	context.Context, string, string, string, time.Time,
) error {
	stub.completed = true
	return nil
}
func (stub *inboxRepositoryStub) Fail(
	context.Context, string, string, string, time.Time, string,
) error {
	stub.failed = true
	return nil
}

type wagerMessageExecutorStub struct {
	command ExecuteWagerTransactionCommand
	err     error
	calls   int
}

func (stub *wagerMessageExecutorStub) Execute(
	_ context.Context, command ExecuteWagerTransactionCommand,
) (ExecuteWagerTransactionResult, error) {
	stub.calls++
	stub.command = command
	return ExecuteWagerTransactionResult{}, stub.err
}

func TestConsumeWagerMessageClaimsProcessesCompletesAndDeletes(t *testing.T) {
	inbox := &inboxRepositoryStub{claimStatus: InboxClaimed}
	executor := &wagerMessageExecutorStub{}
	useCase, err := NewConsumeWagerMessageUseCase(inbox, executor)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	result, err := useCase.Execute(
		context.Background(), validWagerMessage(), "worker-1", now,
		now.Add(30*time.Second), now.Add(time.Second),
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.DeleteFromQueue || result.AlreadyProcessed || !inbox.completed || inbox.failed {
		t.Fatalf("result/inbox = %#v completed=%v failed=%v", result, inbox.completed, inbox.failed)
	}
	if executor.calls != 1 || executor.command.IdempotencyKey != "provider-a:transaction-1" ||
		executor.command.Payload.ProviderID != "provider-a" ||
		executor.command.CausationID != "message-1" {
		t.Fatalf("executor command = %#v", executor.command)
	}
	if inbox.message.PayloadHash == "" || inbox.message.ConsumerName != WagerTransactionsConsumerName {
		t.Fatalf("claimed message = %#v", inbox.message)
	}
}

func TestConsumeWagerMessageDeletesCompletedReplayWithoutExecuting(t *testing.T) {
	inbox := &inboxRepositoryStub{claimStatus: InboxAlreadyProcessed}
	executor := &wagerMessageExecutorStub{}
	useCase, _ := NewConsumeWagerMessageUseCase(inbox, executor)
	now := time.Now().UTC()
	result, err := useCase.Execute(
		context.Background(), validWagerMessage(), "worker-1", now,
		now.Add(time.Minute), now.Add(time.Second),
	)
	if err != nil || !result.DeleteFromQueue || !result.AlreadyProcessed || executor.calls != 0 {
		t.Fatalf("result=%#v calls=%d error=%v", result, executor.calls, err)
	}
}

func TestConsumeWagerMessageReleasesInboxAfterProcessingFailure(t *testing.T) {
	processingErr := errors.New("database unavailable")
	inbox := &inboxRepositoryStub{claimStatus: InboxClaimed}
	executor := &wagerMessageExecutorStub{err: processingErr}
	useCase, _ := NewConsumeWagerMessageUseCase(inbox, executor)
	now := time.Now().UTC()
	result, err := useCase.Execute(
		context.Background(), validWagerMessage(), "worker-1", now,
		now.Add(time.Minute), now.Add(time.Second),
	)
	if !errors.Is(err, processingErr) || result.DeleteFromQueue || !inbox.failed || inbox.completed {
		t.Fatalf("result=%#v failed=%v completed=%v error=%v", result, inbox.failed, inbox.completed, err)
	}
}

func TestConsumeWagerMessageRejectsInvalidEnvelopeBeforeInbox(t *testing.T) {
	inbox := &inboxRepositoryStub{claimStatus: InboxClaimed}
	useCase, _ := NewConsumeWagerMessageUseCase(inbox, &wagerMessageExecutorStub{})
	now := time.Now().UTC()
	_, err := useCase.Execute(
		context.Background(), []byte(`{"messageId":"message-1","type":"Unknown"}`),
		"worker-1", now, now.Add(time.Minute), now.Add(time.Second),
	)
	if !errors.Is(err, ErrInvalidWagerMessage) || inbox.message.MessageID != "" {
		t.Fatalf("error=%v claimed=%#v", err, inbox.message)
	}
}

func TestConsumeWagerMessageRejectsInvalidRetryScheduleBeforeInbox(t *testing.T) {
	inbox := &inboxRepositoryStub{claimStatus: InboxClaimed}
	useCase, _ := NewConsumeWagerMessageUseCase(inbox, &wagerMessageExecutorStub{})
	now := time.Now().UTC()
	_, err := useCase.Execute(
		context.Background(), validWagerMessage(), "worker-1",
		now, now.Add(time.Minute), now,
	)
	if !errors.Is(err, ErrInvalidWagerMessage) || inbox.message.MessageID != "" {
		t.Fatalf("error=%v claimed=%#v", err, inbox.message)
	}
}

func validWagerMessage() []byte {
	return []byte(`{
		"messageId":"message-1",
		"type":"WagerTransactionRequested",
		"occurredAt":"2026-10-01T12:00:00Z",
		"data":{
			"providerId":"provider-a",
			"externalTransactionId":"transaction-1",
			"idempotencyKey":"provider-a:transaction-1",
			"playerId":"player-1",
			"walletId":"wallet-1",
			"roundId":"round-1",
			"gameId":"game-1",
			"kind":"BET",
			"money":{"amount":"25.00","currency":"BRL"}
		}
	}`)
}

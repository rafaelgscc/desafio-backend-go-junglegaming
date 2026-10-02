package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type outboxDeliveryRepositoryStub struct {
	events           []OutboxEvent
	claimErr         error
	markPublishedErr error
	published        []string
	failed           []string
	nextAttemptAt    time.Time
}

func (stub *outboxDeliveryRepositoryStub) ClaimOutboxEvents(
	context.Context, string, time.Time, time.Time, int,
) ([]OutboxEvent, error) {
	return stub.events, stub.claimErr
}
func (stub *outboxDeliveryRepositoryStub) MarkOutboxEventPublished(
	_ context.Context, eventID, _ string, _ time.Time,
) error {
	stub.published = append(stub.published, eventID)
	return stub.markPublishedErr
}
func (stub *outboxDeliveryRepositoryStub) MarkOutboxEventFailed(
	_ context.Context, eventID, _ string, nextAttemptAt time.Time, _ string,
) error {
	stub.failed = append(stub.failed, eventID)
	stub.nextAttemptAt = nextAttemptAt
	return nil
}

type integrationEventPublisherStub struct {
	published []string
	failID    string
}

func (stub *integrationEventPublisherStub) Publish(
	_ context.Context, event OutboxEvent,
) error {
	if event.EventID == stub.failID {
		return errors.New("SQS unavailable")
	}
	stub.published = append(stub.published, event.EventID)
	return nil
}

func TestPublishOutboxBatchPublishesAndConfirmsClaimedEvents(t *testing.T) {
	repository := &outboxDeliveryRepositoryStub{events: []OutboxEvent{
		{EventID: "event-1", Data: json.RawMessage(`{"value":1}`), PublishAttempts: 1},
		{EventID: "event-2", Data: json.RawMessage(`{"value":2}`), PublishAttempts: 1},
	}}
	publisher := &integrationEventPublisherStub{}
	useCase, err := NewPublishOutboxBatchUseCase(repository, publisher)
	if err != nil {
		t.Fatal(err)
	}
	result, err := useCase.Execute(context.Background(), validPublishOutboxCommand())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Claimed != 2 || result.Published != 2 || result.Failed != 0 ||
		len(repository.published) != 2 || len(publisher.published) != 2 {
		t.Fatalf("result/repository/publisher = %#v/%#v/%#v", result, repository.published, publisher.published)
	}
}

func TestPublishOutboxBatchSchedulesFailedPublicationWithBackoff(t *testing.T) {
	now := validPublishOutboxCommand().Now
	repository := &outboxDeliveryRepositoryStub{events: []OutboxEvent{
		{EventID: "event-1", PublishAttempts: 3},
	}}
	useCase, _ := NewPublishOutboxBatchUseCase(
		repository, &integrationEventPublisherStub{failID: "event-1"},
	)
	result, err := useCase.Execute(context.Background(), validPublishOutboxCommand())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Failed != 1 || len(repository.failed) != 1 ||
		!repository.nextAttemptAt.Equal(now.Add(4*time.Second)) {
		t.Fatalf("result/failed/next = %#v/%#v/%v", result, repository.failed, repository.nextAttemptAt)
	}
}

func TestPublishOutboxBatchLeavesLeaseForRecoveryWhenConfirmationFails(t *testing.T) {
	confirmationErr := errors.New("database unavailable after publish")
	repository := &outboxDeliveryRepositoryStub{
		events:           []OutboxEvent{{EventID: "event-1", PublishAttempts: 1}},
		markPublishedErr: confirmationErr,
	}
	useCase, _ := NewPublishOutboxBatchUseCase(repository, &integrationEventPublisherStub{})
	result, err := useCase.Execute(context.Background(), validPublishOutboxCommand())
	if !errors.Is(err, confirmationErr) || result.Published != 0 || len(repository.failed) != 0 {
		t.Fatalf("result=%#v failed=%#v error=%v", result, repository.failed, err)
	}
}

func TestPublishOutboxBatchRejectsInvalidCommand(t *testing.T) {
	useCase, _ := NewPublishOutboxBatchUseCase(
		&outboxDeliveryRepositoryStub{}, &integrationEventPublisherStub{},
	)
	_, err := useCase.Execute(context.Background(), PublishOutboxCommand{})
	if !errors.Is(err, ErrInvalidPublishOutboxCommand) {
		t.Fatalf("Execute() error = %v", err)
	}
}

func validPublishOutboxCommand() PublishOutboxCommand {
	return PublishOutboxCommand{
		WorkerID:      "publisher-1",
		Now:           time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC),
		LeaseDuration: 30 * time.Second,
		BatchSize:     20,
	}
}

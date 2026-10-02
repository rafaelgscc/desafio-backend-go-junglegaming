package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrOutboxDeliveryRepositoryRequired  = errors.New("outbox delivery repository is required")
	ErrIntegrationEventPublisherRequired = errors.New("integration event publisher is required")
	ErrInvalidPublishOutboxCommand       = errors.New("invalid publish outbox command")
	ErrOutboxLeaseLost                   = errors.New("outbox event lease lost")
)

type OutboxEvent struct {
	EventID         string
	EventType       IntegrationEventType
	AggregateID     string
	CorrelationID   string
	CausationID     string
	OccurredAt      time.Time
	Version         int
	Data            json.RawMessage
	PublishAttempts int
}

type OutboxDeliveryRepository interface {
	ClaimOutboxEvents(
		context.Context,
		string,
		time.Time,
		time.Time,
		int,
	) ([]OutboxEvent, error)
	MarkOutboxEventPublished(context.Context, string, string, time.Time) error
	MarkOutboxEventFailed(context.Context, string, string, time.Time, string) error
}

type IntegrationEventPublisher interface {
	Publish(context.Context, OutboxEvent) error
}

type PublishOutboxCommand struct {
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	BatchSize     int
}

type PublishOutboxResult struct {
	Claimed   int
	Published int
	Failed    int
}

type PublishOutboxBatchUseCase struct {
	repository OutboxDeliveryRepository
	publisher  IntegrationEventPublisher
}

func NewPublishOutboxBatchUseCase(
	repository OutboxDeliveryRepository,
	publisher IntegrationEventPublisher,
) (*PublishOutboxBatchUseCase, error) {
	if repository == nil {
		return nil, ErrOutboxDeliveryRepositoryRequired
	}
	if publisher == nil {
		return nil, ErrIntegrationEventPublisherRequired
	}
	return &PublishOutboxBatchUseCase{repository: repository, publisher: publisher}, nil
}

func (useCase *PublishOutboxBatchUseCase) Execute(
	ctx context.Context,
	command PublishOutboxCommand,
) (PublishOutboxResult, error) {
	if command.WorkerID == "" || command.Now.IsZero() ||
		command.LeaseDuration <= 0 || command.BatchSize < 1 || command.BatchSize > 100 {
		return PublishOutboxResult{}, ErrInvalidPublishOutboxCommand
	}
	now := command.Now.UTC()
	events, err := useCase.repository.ClaimOutboxEvents(
		ctx, command.WorkerID, now, now.Add(command.LeaseDuration), command.BatchSize,
	)
	if err != nil {
		return PublishOutboxResult{}, err
	}
	result := PublishOutboxResult{Claimed: len(events)}
	for _, event := range events {
		if err := useCase.publisher.Publish(ctx, event); err != nil {
			result.Failed++
			nextAttemptAt := now.Add(outboxRetryBackoff(event.PublishAttempts))
			if markErr := useCase.repository.MarkOutboxEventFailed(
				ctx, event.EventID, command.WorkerID, nextAttemptAt, err.Error(),
			); markErr != nil {
				return result, fmt.Errorf("publish outbox event %s: %w; persist failure: %v", event.EventID, err, markErr)
			}
			continue
		}
		if err := useCase.repository.MarkOutboxEventPublished(
			ctx, event.EventID, command.WorkerID, now,
		); err != nil {
			return result, err
		}
		result.Published++
	}
	return result, nil
}

func outboxRetryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 6 {
		shift = 6
	}
	return time.Duration(1<<shift) * time.Second
}

package sqsadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

type EventPublisher struct {
	transport *Transport
}

func NewEventPublisher(transport *Transport) (*EventPublisher, error) {
	if transport == nil {
		return nil, ErrTransportRequired
	}
	return &EventPublisher{transport: transport}, nil
}

type integrationEventEnvelope struct {
	EventID       string                           `json:"eventId"`
	EventType     application.IntegrationEventType `json:"eventType"`
	AggregateID   string                           `json:"aggregateId"`
	CorrelationID string                           `json:"correlationId"`
	CausationID   string                           `json:"causationId,omitempty"`
	OccurredAt    time.Time                        `json:"occurredAt"`
	Version       int                              `json:"version"`
	Data          json.RawMessage                  `json:"data"`
}

func (publisher *EventPublisher) Publish(
	ctx context.Context,
	event application.OutboxEvent,
) error {
	body, err := json.Marshal(integrationEventEnvelope{
		EventID: event.EventID, EventType: event.EventType,
		AggregateID: event.AggregateID, CorrelationID: event.CorrelationID,
		CausationID: event.CausationID, OccurredAt: event.OccurredAt.UTC(),
		Version: event.Version, Data: event.Data,
	})
	if err != nil {
		return fmt.Errorf("encode integration event: %w", err)
	}
	_, err = publisher.transport.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:               aws.String(publisher.transport.eventQueueURL),
		MessageBody:            aws.String(string(body)),
		MessageGroupId:         aws.String(event.AggregateID),
		MessageDeduplicationId: aws.String(event.EventID),
	})
	if err != nil {
		return fmt.Errorf("publish integration event: %w", err)
	}
	return nil
}

var _ application.IntegrationEventPublisher = (*EventPublisher)(nil)

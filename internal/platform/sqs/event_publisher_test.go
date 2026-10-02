package sqsadapter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/observability"
)

func TestEventPublisherPreservesEventIdentityAndRouting(t *testing.T) {
	client := &sqsClientStub{}
	publisher, err := NewEventPublisher(&Transport{
		client: client, eventQueueURL: "http://sqs/integration-events.fifo",
	}, observability.NewMetrics())
	if err != nil {
		t.Fatal(err)
	}
	event := application.OutboxEvent{
		EventID: "event-1", EventType: application.IntegrationEventTypeWalletBalanceChanged,
		AggregateID: "wallet-1", CorrelationID: "correlation-1", CausationID: "message-1",
		OccurredAt: time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC), Version: 1,
		Data: json.RawMessage(`{"walletId":"wallet-1"}`),
	}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	client.mu.Lock()
	sent := client.sent
	client.mu.Unlock()
	if sent == nil || aws.ToString(sent.QueueUrl) != "http://sqs/integration-events.fifo" ||
		aws.ToString(sent.MessageGroupId) != "wallet-1" ||
		aws.ToString(sent.MessageDeduplicationId) != "event-1" {
		t.Fatalf("SendMessage input = %#v", sent)
	}
	var envelope struct {
		EventID       string                           `json:"eventId"`
		EventType     application.IntegrationEventType `json:"eventType"`
		AggregateID   string                           `json:"aggregateId"`
		CorrelationID string                           `json:"correlationId"`
		CausationID   string                           `json:"causationId"`
		OccurredAt    string                           `json:"occurredAt"`
		Version       int                              `json:"version"`
		Data          map[string]any                   `json:"data"`
	}
	if err := json.Unmarshal([]byte(aws.ToString(sent.MessageBody)), &envelope); err != nil {
		t.Fatalf("decode published envelope: %v", err)
	}
	if envelope.EventID != "event-1" ||
		envelope.EventType != application.IntegrationEventTypeWalletBalanceChanged ||
		envelope.AggregateID != "wallet-1" || envelope.CorrelationID != "correlation-1" ||
		envelope.CausationID != "message-1" || envelope.OccurredAt != "2026-10-02T12:00:00Z" ||
		envelope.Version != 1 || envelope.Data["walletId"] != "wallet-1" {
		t.Fatalf("published envelope = %#v", envelope)
	}
}

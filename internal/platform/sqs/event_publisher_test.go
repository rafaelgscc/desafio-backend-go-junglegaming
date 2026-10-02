package sqsadapter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

func TestEventPublisherPreservesEventIdentityAndRouting(t *testing.T) {
	client := &sqsClientStub{}
	publisher, err := NewEventPublisher(&Transport{
		client: client, eventQueueURL: "http://sqs/integration-events.fifo",
	})
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
	if client.sent == nil || aws.ToString(client.sent.QueueUrl) != "http://sqs/integration-events.fifo" ||
		aws.ToString(client.sent.MessageGroupId) != "wallet-1" ||
		aws.ToString(client.sent.MessageDeduplicationId) != "event-1" {
		t.Fatalf("SendMessage input = %#v", client.sent)
	}
	body := aws.ToString(client.sent.MessageBody)
	for _, expected := range []string{
		`"eventId":"event-1"`, `"eventType":"WalletBalanceChanged"`,
		`"aggregateId":"wallet-1"`, `"data":{"walletId":"wallet-1"}`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("published body %s does not contain %s", body, expected)
		}
	}
}

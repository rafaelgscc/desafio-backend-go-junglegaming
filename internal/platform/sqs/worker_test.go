package sqsadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

type sqsClientStub struct {
	deleted           int
	visibilityChanges int
	visibilityTimeout int32
}

func (stub *sqsClientStub) GetQueueUrl(
	context.Context, *sqs.GetQueueUrlInput, ...func(*sqs.Options),
) (*sqs.GetQueueUrlOutput, error) {
	return &sqs.GetQueueUrlOutput{QueueUrl: aws.String("http://sqs/queue")}, nil
}
func (stub *sqsClientStub) GetQueueAttributes(
	context.Context, *sqs.GetQueueAttributesInput, ...func(*sqs.Options),
) (*sqs.GetQueueAttributesOutput, error) {
	return &sqs.GetQueueAttributesOutput{}, nil
}
func (stub *sqsClientStub) ReceiveMessage(
	context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options),
) (*sqs.ReceiveMessageOutput, error) {
	return &sqs.ReceiveMessageOutput{}, nil
}
func (stub *sqsClientStub) DeleteMessage(
	context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options),
) (*sqs.DeleteMessageOutput, error) {
	stub.deleted++
	return &sqs.DeleteMessageOutput{}, nil
}
func (stub *sqsClientStub) ChangeMessageVisibility(
	_ context.Context, input *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options),
) (*sqs.ChangeMessageVisibilityOutput, error) {
	stub.visibilityChanges++
	stub.visibilityTimeout = input.VisibilityTimeout
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

type messageConsumerStub struct {
	result application.ConsumeWagerMessageResult
	err    error
}

func (stub *messageConsumerStub) Execute(
	context.Context, []byte, string, time.Time, time.Time, time.Time,
) (application.ConsumeWagerMessageResult, error) {
	return stub.result, stub.err
}

func TestWorkerDeletesOnlyDurablyCompletedMessages(t *testing.T) {
	client := &sqsClientStub{}
	worker := &Worker{
		transport: &Transport{client: client, queueURL: "http://sqs/queue"},
		consumer: &messageConsumerStub{result: application.ConsumeWagerMessageResult{
			MessageID: "message-1", DeleteFromQueue: true,
		}},
		config: config.SQSConfig{WorkerID: "worker-1", VisibilityTimeout: 30 * time.Second},
	}
	worker.processMessage(context.Background(), types.Message{
		Body: aws.String(`{}`), ReceiptHandle: aws.String("receipt-1"),
		Attributes: map[string]string{"ApproximateReceiveCount": "1"},
	})
	if client.deleted != 1 || client.visibilityChanges != 0 {
		t.Fatalf("deleted/visibility = %d/%d", client.deleted, client.visibilityChanges)
	}
}

func TestWorkerAppliesExponentialBackoffAfterFailure(t *testing.T) {
	client := &sqsClientStub{}
	worker := &Worker{
		transport: &Transport{client: client, queueURL: "http://sqs/queue"},
		consumer:  &messageConsumerStub{err: errors.New("temporary failure")},
		config:    config.SQSConfig{WorkerID: "worker-1", VisibilityTimeout: 30 * time.Second},
	}
	worker.processMessage(context.Background(), types.Message{
		Body: aws.String(`{}`), ReceiptHandle: aws.String("receipt-1"),
		Attributes: map[string]string{"ApproximateReceiveCount": "4"},
	})
	if client.deleted != 0 || client.visibilityChanges != 1 || client.visibilityTimeout != 8 {
		t.Fatalf("deleted/visibility/timeout = %d/%d/%d", client.deleted, client.visibilityChanges, client.visibilityTimeout)
	}
}

func TestRetryBackoffIsBounded(t *testing.T) {
	if retryBackoff(1) != time.Second || retryBackoff(10) != 32*time.Second {
		t.Fatalf("backoff = %v/%v", retryBackoff(1), retryBackoff(10))
	}
}

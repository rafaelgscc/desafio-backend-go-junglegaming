package sqsadapter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

type sqsClientStub struct {
	mu                sync.Mutex
	deleted           int
	visibilityChanges int
	visibilityTimeout int32
	sent              *sqs.SendMessageInput
	sendErr           error
}

func (stub *sqsClientStub) SendMessage(
	_ context.Context, input *sqs.SendMessageInput, _ ...func(*sqs.Options),
) (*sqs.SendMessageOutput, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.sent = input
	return &sqs.SendMessageOutput{}, stub.sendErr
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
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.deleted++
	return &sqs.DeleteMessageOutput{}, nil
}
func (stub *sqsClientStub) ChangeMessageVisibility(
	_ context.Context, input *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options),
) (*sqs.ChangeMessageVisibilityOutput, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.visibilityChanges++
	stub.visibilityTimeout = input.VisibilityTimeout
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

type messageConsumerStub struct {
	result application.ConsumeWagerMessageResult
	err    error
}

type groupedMessageConsumerStub struct {
	mu               sync.Mutex
	activeByGroup    map[byte]int
	active           int
	maxActive        int
	sameGroupOverlap bool
	order            map[byte][]string
	started          chan struct{}
	release          chan struct{}
}

type receiveOnceSQSClient struct {
	*sqsClientStub
	mu       sync.Mutex
	received bool
}

func (client *receiveOnceSQSClient) ReceiveMessage(
	ctx context.Context,
	_ *sqs.ReceiveMessageInput,
	_ ...func(*sqs.Options),
) (*sqs.ReceiveMessageOutput, error) {
	client.mu.Lock()
	if !client.received {
		client.received = true
		client.mu.Unlock()
		return &sqs.ReceiveMessageOutput{Messages: []types.Message{{
			Body: aws.String("message-1"), ReceiptHandle: aws.String("receipt-1"),
			Attributes: map[string]string{
				"ApproximateReceiveCount": "1", "MessageGroupId": "wallet-1",
			},
		}}}, nil
	}
	client.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

type blockingMessageConsumer struct {
	started chan struct{}
	release chan struct{}
}

func (consumer *blockingMessageConsumer) Execute(
	context.Context, []byte, string, time.Time, time.Time, time.Time,
) (application.ConsumeWagerMessageResult, error) {
	close(consumer.started)
	<-consumer.release
	return application.ConsumeWagerMessageResult{
		MessageID: "message-1", DeleteFromQueue: true,
	}, nil
}

func (stub *groupedMessageConsumerStub) Execute(
	_ context.Context,
	body []byte,
	_ string,
	_ time.Time,
	_ time.Time,
	_ time.Time,
) (application.ConsumeWagerMessageResult, error) {
	group := body[0]
	stub.mu.Lock()
	if stub.activeByGroup[group] > 0 {
		stub.sameGroupOverlap = true
	}
	stub.activeByGroup[group]++
	stub.active++
	if stub.active > stub.maxActive {
		stub.maxActive = stub.active
	}
	stub.order[group] = append(stub.order[group], string(body))
	stub.mu.Unlock()
	stub.started <- struct{}{}
	<-stub.release
	stub.mu.Lock()
	stub.activeByGroup[group]--
	stub.active--
	stub.mu.Unlock()
	return application.ConsumeWagerMessageResult{DeleteFromQueue: true}, nil
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

func TestWorkerReleasesMessageImmediatelyAfterForcedShutdown(t *testing.T) {
	client := &sqsClientStub{}
	worker := &Worker{
		transport: &Transport{client: client, queueURL: "http://sqs/queue"},
		consumer:  &messageConsumerStub{err: context.Canceled},
		config:    config.SQSConfig{WorkerID: "worker-1", VisibilityTimeout: 30 * time.Second},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker.processMessage(ctx, types.Message{
		Body: aws.String(`{}`), ReceiptHandle: aws.String("receipt-1"),
		Attributes: map[string]string{"ApproximateReceiveCount": "2"},
	})
	if client.deleted != 0 || client.visibilityChanges != 1 || client.visibilityTimeout != 0 {
		t.Fatalf(
			"deleted/visibility/timeout = %d/%d/%d",
			client.deleted, client.visibilityChanges, client.visibilityTimeout,
		)
	}
}

func TestWorkerPreservesFIFOOrderWithinGroupAndProcessesGroupsInParallel(t *testing.T) {
	consumer := &groupedMessageConsumerStub{
		activeByGroup: make(map[byte]int), order: make(map[byte][]string),
		started: make(chan struct{}, 3), release: make(chan struct{}),
	}
	worker := &Worker{
		transport: &Transport{client: &sqsClientStub{}, queueURL: "http://sqs/queue"},
		consumer:  consumer,
	}
	done := make(chan struct{})
	go func() {
		worker.processByGroup(context.Background(), []types.Message{
			{Body: aws.String("a1"), Attributes: map[string]string{"MessageGroupId": "wallet-a"}},
			{Body: aws.String("a2"), Attributes: map[string]string{"MessageGroupId": "wallet-a"}},
			{Body: aws.String("b1"), Attributes: map[string]string{"MessageGroupId": "wallet-b"}},
		})
		close(done)
	}()
	for range 2 {
		select {
		case <-consumer.started:
		case <-time.After(time.Second):
			t.Fatal("different message groups did not begin in parallel")
		}
	}
	consumer.mu.Lock()
	maxActive := consumer.maxActive
	consumer.mu.Unlock()
	if maxActive != 2 {
		t.Fatalf("maximum parallel messages = %d, want 2 groups", maxActive)
	}
	close(consumer.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("group processing did not finish")
	}
	consumer.mu.Lock()
	defer consumer.mu.Unlock()
	if consumer.sameGroupOverlap {
		t.Fatal("messages from the same FIFO group overlapped")
	}
	if len(consumer.order['a']) != 2 || consumer.order['a'][0] != "a1" || consumer.order['a'][1] != "a2" {
		t.Fatalf("wallet-a processing order = %v", consumer.order['a'])
	}
}

func TestWorkerStopsPollingAndFinishesInFlightMessage(t *testing.T) {
	client := &receiveOnceSQSClient{sqsClientStub: &sqsClientStub{}}
	consumer := &blockingMessageConsumer{
		started: make(chan struct{}), release: make(chan struct{}),
	}
	worker := &Worker{
		transport: &Transport{client: client, queueURL: "http://sqs/queue"},
		consumer:  consumer,
		config: config.SQSConfig{
			WorkerID: "worker-1", MaxMessages: 10,
			WaitTime: time.Second, VisibilityTimeout: 30 * time.Second,
		},
	}
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-consumer.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not begin message processing")
	}
	stopContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- worker.Stop(stopContext) }()
	select {
	case err := <-stopped:
		t.Fatalf("worker stopped before in-flight message completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(consumer.release)
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after in-flight message completed")
	}
	client.sqsClientStub.mu.Lock()
	deleted := client.deleted
	client.sqsClientStub.mu.Unlock()
	if deleted != 1 {
		t.Fatalf("deleted messages = %d, want 1", deleted)
	}
}

func TestRetryBackoffIsBounded(t *testing.T) {
	if retryBackoff(1) != time.Second || retryBackoff(10) != 32*time.Second {
		t.Fatalf("backoff = %v/%v", retryBackoff(1), retryBackoff(10))
	}
}

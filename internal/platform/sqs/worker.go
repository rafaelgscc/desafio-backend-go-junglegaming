package sqsadapter

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

type WagerMessageConsumer interface {
	Execute(context.Context, []byte, string, time.Time, time.Time, time.Time) (application.ConsumeWagerMessageResult, error)
}

type Worker struct {
	transport     *Transport
	consumer      WagerMessageConsumer
	config        config.SQSConfig
	pollCancel    context.CancelFunc
	processCancel context.CancelFunc
	done          chan struct{}
	mu            sync.Mutex
}

func NewWorker(
	transport *Transport,
	consumer *application.ConsumeWagerMessageUseCase,
	sqsConfig config.SQSConfig,
) (*Worker, error) {
	if transport == nil {
		return nil, ErrTransportRequired
	}
	if consumer == nil {
		return nil, application.ErrWagerMessageExecutorRequired
	}
	return &Worker{transport: transport, consumer: consumer, config: sqsConfig}, nil
}

func (worker *Worker) Start(context.Context) error {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.pollCancel != nil {
		return nil
	}
	processContext, processCancel := context.WithCancel(context.Background())
	pollContext, pollCancel := context.WithCancel(processContext)
	worker.pollCancel = pollCancel
	worker.processCancel = processCancel
	worker.done = make(chan struct{})
	go worker.run(pollContext, processContext)
	return nil
}

func (worker *Worker) Stop(ctx context.Context) error {
	worker.mu.Lock()
	pollCancel, processCancel, done := worker.pollCancel, worker.processCancel, worker.done
	worker.mu.Unlock()
	if pollCancel == nil {
		return nil
	}
	pollCancel()
	select {
	case <-done:
		processCancel()
		return nil
	case <-ctx.Done():
		processCancel()
		return ctx.Err()
	}
}

func (worker *Worker) run(pollContext, processContext context.Context) {
	defer close(worker.done)
	for pollContext.Err() == nil {
		output, err := worker.transport.client.ReceiveMessage(pollContext, &sqs.ReceiveMessageInput{
			QueueUrl: aws.String(worker.transport.queueURL), MaxNumberOfMessages: worker.config.MaxMessages,
			WaitTimeSeconds:   int32(worker.config.WaitTime / time.Second),
			VisibilityTimeout: int32(worker.config.VisibilityTimeout / time.Second),
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{
				types.MessageSystemAttributeNameApproximateReceiveCount,
				types.MessageSystemAttributeNameMessageGroupId,
			},
		})
		if err != nil {
			if pollContext.Err() == nil {
				slog.Error("sqs receive failed", "error", err)
				select {
				case <-time.After(time.Second):
				case <-pollContext.Done():
				}
			}
			continue
		}
		worker.processByGroup(processContext, output.Messages)
	}
}

func (worker *Worker) processByGroup(ctx context.Context, messages []types.Message) {
	groups := make(map[string][]types.Message)
	for _, message := range messages {
		groupID := message.Attributes[string(types.MessageSystemAttributeNameMessageGroupId)]
		groups[groupID] = append(groups[groupID], message)
	}
	var waitGroup sync.WaitGroup
	for _, group := range groups {
		group := group
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for _, message := range group {
				if ctx.Err() != nil {
					worker.releaseMessage(message)
					continue
				}
				worker.processMessage(ctx, message)
			}
		}()
	}
	waitGroup.Wait()
}

func (worker *Worker) processMessage(ctx context.Context, message types.Message) {
	now := time.Now().UTC()
	receiveCount, _ := strconv.Atoi(message.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
	if receiveCount < 1 {
		receiveCount = 1
	}
	retryDelay := retryBackoff(receiveCount)
	result, err := worker.consumer.Execute(
		ctx, []byte(aws.ToString(message.Body)), worker.config.WorkerID, now,
		now.Add(worker.config.VisibilityTimeout), now.Add(retryDelay),
	)
	if err == nil && result.DeleteFromQueue {
		_, deleteErr := worker.transport.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl: worker.transportQueueURL(), ReceiptHandle: message.ReceiptHandle,
		})
		if deleteErr != nil && ctx.Err() == nil {
			slog.Error("sqs delete failed", "messageId", result.MessageID, "error", deleteErr)
		} else if deleteErr != nil {
			worker.releaseMessage(message)
		}
		return
	}
	if errors.Is(err, application.ErrInboxMessageBusy) {
		retryDelay = time.Second
	}
	visibilityContext := ctx
	cancelVisibility := func() {}
	if ctx.Err() != nil {
		// The shutdown deadline interrupted processing. Release the message with a
		// fresh bounded context so another instance can resume it immediately.
		retryDelay = 0
		visibilityContext, cancelVisibility = context.WithTimeout(context.Background(), 2*time.Second)
	}
	defer cancelVisibility()
	_, visibilityErr := worker.transport.client.ChangeMessageVisibility(
		visibilityContext,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl: worker.transportQueueURL(), ReceiptHandle: message.ReceiptHandle,
			VisibilityTimeout: int32(retryDelay / time.Second),
		},
	)
	if visibilityErr != nil {
		slog.Error("sqs visibility change failed", "messageId", result.MessageID, "error", visibilityErr)
	}
}

func (worker *Worker) releaseMessage(message types.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := worker.transport.client.ChangeMessageVisibility(
		ctx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl: worker.transportQueueURL(), ReceiptHandle: message.ReceiptHandle,
			VisibilityTimeout: 0,
		},
	)
	if err != nil {
		slog.Error("sqs message release failed", "error", err)
	}
}

func (worker *Worker) transportQueueURL() *string {
	return aws.String(worker.transport.queueURL)
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 5 {
		shift = 5
	}
	return time.Duration(1<<shift) * time.Second
}

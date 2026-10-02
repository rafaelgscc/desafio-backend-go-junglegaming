package outboxworker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/observability"
)

type emptyOutboxRepository struct {
	calls atomic.Int32
}

func (repository *emptyOutboxRepository) ClaimOutboxEvents(
	context.Context, string, time.Time, time.Time, int,
) ([]application.OutboxEvent, error) {
	repository.calls.Add(1)
	return nil, nil
}
func (*emptyOutboxRepository) MarkOutboxEventPublished(context.Context, string, string, time.Time) error {
	return nil
}
func (*emptyOutboxRepository) MarkOutboxEventFailed(context.Context, string, string, time.Time, string) error {
	return nil
}

type noOpEventPublisher struct{}

func (noOpEventPublisher) Publish(context.Context, application.OutboxEvent) error { return nil }

func TestWorkerStartsPollsAndStops(t *testing.T) {
	repository := &emptyOutboxRepository{}
	useCase, _ := application.NewPublishOutboxBatchUseCase(repository, noOpEventPublisher{})
	worker, err := NewWorker(useCase, config.OutboxConfig{
		WorkerID: "publisher-1", BatchSize: 10,
		PollInterval: time.Millisecond, LeaseDuration: time.Second,
	}, observability.NewMetrics())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for repository.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stopContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Stop(stopContext); err != nil {
		t.Fatal(err)
	}
	if repository.calls.Load() == 0 {
		t.Fatal("worker did not poll the outbox")
	}
}

type singleOutboxRepository struct {
	claimed   atomic.Bool
	published chan struct{}
}

func (repository *singleOutboxRepository) ClaimOutboxEvents(
	context.Context, string, time.Time, time.Time, int,
) ([]application.OutboxEvent, error) {
	if repository.claimed.Swap(true) {
		return nil, nil
	}
	return []application.OutboxEvent{{EventID: "event-1", PublishAttempts: 1}}, nil
}

func (repository *singleOutboxRepository) MarkOutboxEventPublished(
	context.Context, string, string, time.Time,
) error {
	close(repository.published)
	return nil
}

func (*singleOutboxRepository) MarkOutboxEventFailed(
	context.Context, string, string, time.Time, string,
) error {
	return nil
}

type blockingEventPublisher struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (publisher *blockingEventPublisher) Publish(
	ctx context.Context, _ application.OutboxEvent,
) error {
	publisher.once.Do(func() { close(publisher.started) })
	select {
	case <-publisher.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestWorkerFinishesInFlightPublicationBeforeStopping(t *testing.T) {
	repository := &singleOutboxRepository{published: make(chan struct{})}
	publisher := &blockingEventPublisher{started: make(chan struct{}), release: make(chan struct{})}
	useCase, _ := application.NewPublishOutboxBatchUseCase(repository, publisher)
	worker, err := NewWorker(useCase, config.OutboxConfig{
		WorkerID: "publisher-1", BatchSize: 10,
		PollInterval: time.Hour, LeaseDuration: time.Second,
	}, observability.NewMetrics())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-publisher.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start publication")
	}

	stopResult := make(chan error, 1)
	stopContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() { stopResult <- worker.Stop(stopContext) }()
	select {
	case err := <-stopResult:
		t.Fatalf("Stop() returned before publication completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(publisher.release)
	if err := <-stopResult; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	select {
	case <-repository.published:
	default:
		t.Fatal("outbox event was not confirmed before shutdown")
	}
}

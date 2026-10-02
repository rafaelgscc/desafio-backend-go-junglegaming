package outboxworker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
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
	})
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

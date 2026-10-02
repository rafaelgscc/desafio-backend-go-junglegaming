package outboxworker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

type Worker struct {
	useCase *application.PublishOutboxBatchUseCase
	config  config.OutboxConfig

	mu            sync.Mutex
	stop          chan struct{}
	done          chan struct{}
	processCancel context.CancelFunc
}

func NewWorker(
	useCase *application.PublishOutboxBatchUseCase,
	outboxConfig config.OutboxConfig,
) (*Worker, error) {
	if useCase == nil {
		return nil, application.ErrOutboxDeliveryRepositoryRequired
	}
	return &Worker{useCase: useCase, config: outboxConfig}, nil
}

func (worker *Worker) Start(context.Context) error {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.stop != nil {
		return nil
	}
	processContext, processCancel := context.WithCancel(context.Background())
	worker.processCancel = processCancel
	worker.stop = make(chan struct{})
	worker.done = make(chan struct{})
	go worker.run(processContext)
	return nil
}

func (worker *Worker) Stop(ctx context.Context) error {
	worker.mu.Lock()
	stop, done, processCancel := worker.stop, worker.done, worker.processCancel
	worker.mu.Unlock()
	if stop == nil {
		return nil
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
	select {
	case <-done:
		processCancel()
		return nil
	case <-ctx.Done():
		processCancel()
		return ctx.Err()
	}
}

func (worker *Worker) run(ctx context.Context) {
	defer close(worker.done)
	for {
		select {
		case <-worker.stop:
			return
		default:
		}
		result, err := worker.useCase.Execute(ctx, application.PublishOutboxCommand{
			WorkerID: worker.config.WorkerID, Now: time.Now().UTC(),
			LeaseDuration: worker.config.LeaseDuration, BatchSize: worker.config.BatchSize,
		})
		if err != nil && ctx.Err() == nil {
			slog.Error("outbox batch failed", "workerId", worker.config.WorkerID, "error", err)
		} else if result.Failed > 0 {
			slog.Warn(
				"outbox events scheduled for retry", "workerId", worker.config.WorkerID,
				"claimed", result.Claimed, "published", result.Published, "failed", result.Failed,
			)
		}
		timer := time.NewTimer(worker.config.PollInterval)
		select {
		case <-worker.stop:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

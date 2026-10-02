package referenceworker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

type Worker struct {
	useCase *application.RetryPendingReferencesUseCase
	config  config.ReferenceWorkerConfig

	mu            sync.Mutex
	stop          chan struct{}
	done          chan struct{}
	processCancel context.CancelFunc
}

func NewWorker(
	useCase *application.RetryPendingReferencesUseCase,
	workerConfig config.ReferenceWorkerConfig,
) (*Worker, error) {
	if useCase == nil {
		return nil, application.ErrPendingReferenceRepositoryRequired
	}
	if workerConfig.WorkerID == "" || workerConfig.BatchSize < 1 || workerConfig.BatchSize > 100 ||
		workerConfig.PollInterval <= 0 || workerConfig.LeaseDuration <= 0 ||
		workerConfig.RetryBaseDelay <= 0 || workerConfig.MaxAttempts < 1 {
		return nil, application.ErrInvalidRetryPendingReferences
	}
	return &Worker{useCase: useCase, config: workerConfig}, nil
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
		result, err := worker.useCase.Execute(ctx, application.RetryPendingReferencesCommand{
			WorkerID: worker.config.WorkerID, Now: time.Now().UTC(),
			LeaseDuration:  worker.config.LeaseDuration,
			RetryBaseDelay: worker.config.RetryBaseDelay,
			MaxAttempts:    worker.config.MaxAttempts, BatchSize: worker.config.BatchSize,
		})
		if err != nil && ctx.Err() == nil {
			slog.Error("pending reference batch failed", "workerId", worker.config.WorkerID, "error", err)
		} else if result.Failed > 0 {
			slog.Warn(
				"pending references scheduled for retry", "workerId", worker.config.WorkerID,
				"claimed", result.Claimed, "processed", result.Processed,
				"rescheduled", result.Rescheduled, "rejected", result.Rejected,
				"failed", result.Failed,
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

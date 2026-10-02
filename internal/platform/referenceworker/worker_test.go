package referenceworker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

type emptyPendingReferenceRepository struct {
	calls atomic.Int32
}

func (repository *emptyPendingReferenceRepository) ClaimPendingReferences(
	context.Context, string, time.Time, time.Time, int,
) ([]application.PendingReference, error) {
	repository.calls.Add(1)
	return nil, nil
}

func (*emptyPendingReferenceRepository) ReleasePendingReference(context.Context, string, string) error {
	return nil
}

func (*emptyPendingReferenceRepository) FailPendingReference(
	context.Context, string, string, time.Time, string,
) error {
	return nil
}

type noOpReversalProcessor struct{}

func (noOpReversalProcessor) Execute(
	context.Context, application.ProcessReversalCommand,
) (application.ProcessReversalResult, error) {
	return application.ProcessReversalResult{}, nil
}

type noOpPendingReferenceExpirer struct{}

func (noOpPendingReferenceExpirer) Execute(
	context.Context, application.ExpirePendingReferenceCommand,
) (application.ProcessReversalResult, error) {
	return application.ProcessReversalResult{}, nil
}

func TestWorkerStartsPollsAndStops(t *testing.T) {
	repository := &emptyPendingReferenceRepository{}
	useCase, err := application.NewRetryPendingReferencesUseCase(
		repository, noOpReversalProcessor{}, noOpReversalProcessor{},
		noOpPendingReferenceExpirer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := NewWorker(useCase, config.ReferenceWorkerConfig{
		WorkerID: "reference-1", BatchSize: 10, PollInterval: time.Millisecond,
		LeaseDuration: time.Second, RetryBaseDelay: time.Minute, MaxAttempts: 5,
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
		t.Fatal("worker did not poll pending references")
	}
}

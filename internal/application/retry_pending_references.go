package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrPendingReferenceRepositoryRequired = errors.New("pending reference repository is required")
	ErrPendingReferenceExpirerRequired    = errors.New("pending reference expirer is required")
	ErrInvalidRetryPendingReferences      = errors.New("invalid retry pending references command")
	ErrReferenceLeaseLost                 = errors.New("pending reference lease lost")
)

type PendingReference struct {
	TransactionID     string
	Kind              domain.WagerTransactionKind
	ReferenceAttempts int
}

type PendingReferenceRepository interface {
	ClaimPendingReferences(context.Context, string, time.Time, time.Time, int) ([]PendingReference, error)
	ReleasePendingReference(context.Context, string, string) error
	FailPendingReference(context.Context, string, string, time.Time, string) error
}

type PendingReferenceExpirer interface {
	Execute(context.Context, ExpirePendingReferenceCommand) (ProcessReversalResult, error)
}

type ExpirePendingReferenceCommand struct {
	TransactionID  string
	OutcomeEventID string
	CorrelationID  string
	CausationID    string
	ExpiredAt      time.Time
}

type ExpirePendingReferenceUseCase struct {
	unitOfWork WageringUnitOfWork
}

func NewExpirePendingReferenceUseCase(
	unitOfWork WageringUnitOfWork,
) (*ExpirePendingReferenceUseCase, error) {
	if unitOfWork == nil {
		return nil, ErrNilWageringUnitOfWork
	}
	return &ExpirePendingReferenceUseCase{unitOfWork: unitOfWork}, nil
}

func (useCase *ExpirePendingReferenceUseCase) Execute(
	ctx context.Context,
	command ExpirePendingReferenceCommand,
) (ProcessReversalResult, error) {
	if command.TransactionID == "" || command.OutcomeEventID == "" ||
		command.CorrelationID == "" || command.ExpiredAt.IsZero() {
		return ProcessReversalResult{}, ErrInvalidRetryPendingReferences
	}
	var result ProcessReversalResult
	err := useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		transaction, err := tx.FindWagerTransactionForUpdate(ctx, command.TransactionID)
		if err != nil {
			return err
		}
		if transaction.Status() != domain.WagerTransactionStatusPendingReference ||
			(transaction.Kind() != domain.WagerTransactionKindRefund &&
				transaction.Kind() != domain.WagerTransactionKindRollback) {
			return domain.ErrInvalidWagerTransactionTransition
		}
		if err := transaction.MarkRejected(
			domain.WagerTransactionFailureCodeReferenceNotFound,
			command.ExpiredAt,
		); err != nil {
			return err
		}
		if err := tx.SaveWagerTransaction(ctx, transaction); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, newWagerTransactionRejectedEvent(
			command.OutcomeEventID, command.CorrelationID, command.CausationID,
			command.ExpiredAt, transaction,
		)); err != nil {
			return err
		}
		result = ProcessReversalResult{
			Status: transaction.Status(), FailureCode: transaction.FailureCode(),
		}
		return nil
	})
	return result, err
}

type RetryPendingReferencesCommand struct {
	WorkerID       string
	Now            time.Time
	LeaseDuration  time.Duration
	RetryBaseDelay time.Duration
	MaxAttempts    int
	BatchSize      int
}

type RetryPendingReferencesResult struct {
	Claimed     int
	Processed   int
	Rescheduled int
	Rejected    int
	Failed      int
}

type RetryPendingReferencesUseCase struct {
	repository PendingReferenceRepository
	refund     ReversalProcessor
	rollback   ReversalProcessor
	expirer    PendingReferenceExpirer
}

func NewRetryPendingReferencesUseCase(
	repository PendingReferenceRepository,
	refund ReversalProcessor,
	rollback ReversalProcessor,
	expirer PendingReferenceExpirer,
) (*RetryPendingReferencesUseCase, error) {
	if repository == nil {
		return nil, ErrPendingReferenceRepositoryRequired
	}
	if refund == nil || rollback == nil {
		return nil, ErrWagerTransactionExecutorRequired
	}
	if expirer == nil {
		return nil, ErrPendingReferenceExpirerRequired
	}
	return &RetryPendingReferencesUseCase{
		repository: repository, refund: refund, rollback: rollback, expirer: expirer,
	}, nil
}

func (useCase *RetryPendingReferencesUseCase) Execute(
	ctx context.Context,
	command RetryPendingReferencesCommand,
) (RetryPendingReferencesResult, error) {
	if command.WorkerID == "" || command.Now.IsZero() || command.LeaseDuration <= 0 ||
		command.RetryBaseDelay <= 0 || command.MaxAttempts < 1 ||
		command.BatchSize < 1 || command.BatchSize > 100 {
		return RetryPendingReferencesResult{}, ErrInvalidRetryPendingReferences
	}
	now := command.Now.UTC()
	pending, err := useCase.repository.ClaimPendingReferences(
		ctx, command.WorkerID, now, now.Add(command.LeaseDuration), command.BatchSize,
	)
	if err != nil {
		return RetryPendingReferencesResult{}, err
	}
	result := RetryPendingReferencesResult{Claimed: len(pending)}
	for _, reference := range pending {
		processResult, err := useCase.process(ctx, command, reference)
		if err != nil {
			result.Failed++
			nextAttemptAt := now.Add(referenceRetryBackoff(
				reference.ReferenceAttempts, command.RetryBaseDelay,
			))
			if failErr := useCase.repository.FailPendingReference(
				ctx, reference.TransactionID, command.WorkerID, nextAttemptAt, err.Error(),
			); failErr != nil {
				return result, fmt.Errorf("retry pending reference %s: %w; release lease: %v", reference.TransactionID, err, failErr)
			}
			continue
		}
		if err := useCase.repository.ReleasePendingReference(
			ctx, reference.TransactionID, command.WorkerID,
		); err != nil {
			return result, err
		}
		switch processResult.Status {
		case domain.WagerTransactionStatusProcessed:
			result.Processed++
		case domain.WagerTransactionStatusPendingReference:
			result.Rescheduled++
		case domain.WagerTransactionStatusRejected:
			result.Rejected++
		}
	}
	return result, nil
}

func (useCase *RetryPendingReferencesUseCase) process(
	ctx context.Context,
	command RetryPendingReferencesCommand,
	reference PendingReference,
) (ProcessReversalResult, error) {
	ids := pendingReferenceIDs(reference.TransactionID, reference.ReferenceAttempts)
	correlationID := "reference-retry:" + reference.TransactionID
	if reference.ReferenceAttempts >= command.MaxAttempts {
		return useCase.expirer.Execute(ctx, ExpirePendingReferenceCommand{
			TransactionID: reference.TransactionID, OutcomeEventID: ids.outcome,
			CorrelationID: correlationID, CausationID: reference.TransactionID,
			ExpiredAt: command.Now.UTC(),
		})
	}
	processor := useCase.refund
	if reference.Kind == domain.WagerTransactionKindRollback {
		processor = useCase.rollback
	} else if reference.Kind != domain.WagerTransactionKindRefund {
		return ProcessReversalResult{}, ErrInvalidReversalReferenceKind
	}
	return processor.Execute(ctx, ProcessReversalCommand{
		TransactionID: reference.TransactionID, LedgerEntryID: ids.ledger,
		OutcomeEventID: ids.outcome, BalanceChangedEventID: ids.balance,
		CorrelationID: correlationID, CausationID: reference.TransactionID,
		ProcessedAt: command.Now.UTC(),
		NextReferenceAttemptAt: command.Now.UTC().Add(referenceRetryBackoff(
			reference.ReferenceAttempts, command.RetryBaseDelay,
		)),
	})
}

type referenceRetryIDs struct {
	ledger, outcome, balance string
}

func pendingReferenceIDs(transactionID string, attempt int) referenceRetryIDs {
	stable := func(suffix string) string {
		digest := sha256.Sum256([]byte(fmt.Sprintf(
			"%s:reference-attempt:%d:%s", transactionID, attempt, suffix,
		)))
		return hex.EncodeToString(digest[:16])
	}
	return referenceRetryIDs{
		ledger: stable("ledger"), outcome: stable("outcome"), balance: stable("balance"),
	}
}

func referenceRetryBackoff(attempt int, base time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt
	if shift > 6 {
		shift = 6
	}
	return time.Duration(1<<shift) * base
}

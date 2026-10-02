package application

import (
	"context"
	"errors"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrIdempotencyKeyConflict      = errors.New("idempotency key reused with different payload")
	ErrExternalTransactionConflict = errors.New("external transaction already exists with another idempotency key")
)

type SubmitWagerTransactionCommand struct {
	TransactionID                  string
	ExternalTransactionID          string
	ProviderID                     string
	IdempotencyKey                 string
	PayloadHash                    string
	WalletID                       string
	PlayerID                       string
	RoundID                        string
	GameID                         string
	Kind                           domain.WagerTransactionKind
	Money                          domain.Money
	ReferenceExternalTransactionID string
	OccurredAt                     time.Time
}

type SubmitWagerTransactionResult struct {
	TransactionID    string
	Status           domain.WagerTransactionStatus
	Balance          domain.Money
	HasBalance       bool
	FailureCode      domain.WagerTransactionFailureCode
	IdempotentReplay bool
}

type SubmitWagerTransactionUseCase struct {
	unitOfWork WageringUnitOfWork
}

func NewSubmitWagerTransactionUseCase(
	unitOfWork WageringUnitOfWork,
) (*SubmitWagerTransactionUseCase, error) {
	if unitOfWork == nil {
		return nil, ErrNilWageringUnitOfWork
	}
	return &SubmitWagerTransactionUseCase{unitOfWork: unitOfWork}, nil
}

func (useCase *SubmitWagerTransactionUseCase) Execute(
	ctx context.Context,
	command SubmitWagerTransactionCommand,
) (SubmitWagerTransactionResult, error) {
	newTransaction, err := domain.NewExternalWagerTransaction(
		domain.NewExternalWagerTransactionParams{
			ID:                             command.TransactionID,
			ExternalTransactionID:          command.ExternalTransactionID,
			ProviderID:                     command.ProviderID,
			IdempotencyKey:                 command.IdempotencyKey,
			PayloadHash:                    command.PayloadHash,
			WalletID:                       command.WalletID,
			PlayerID:                       command.PlayerID,
			RoundID:                        command.RoundID,
			GameID:                         command.GameID,
			Kind:                           command.Kind,
			Money:                          command.Money,
			ReferenceExternalTransactionID: command.ReferenceExternalTransactionID,
			OccurredAt:                     command.OccurredAt,
		},
	)
	if err != nil {
		return SubmitWagerTransactionResult{}, err
	}

	var result SubmitWagerTransactionResult
	err = useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		existing, err := tx.FindWagerTransactionByIdempotencyKeyForUpdate(
			ctx,
			command.ProviderID,
			command.IdempotencyKey,
		)
		if err == nil {
			if existing.PayloadHash() != command.PayloadHash {
				return ErrIdempotencyKeyConflict
			}
			result = submitResultFromTransaction(existing, true)
			return nil
		}
		if !errors.Is(err, ErrWagerTransactionNotFound) {
			return err
		}

		_, err = tx.FindWagerTransactionByExternalIDForUpdate(
			ctx,
			command.ProviderID,
			command.ExternalTransactionID,
		)
		if err == nil {
			return ErrExternalTransactionConflict
		}
		if !errors.Is(err, ErrWagerTransactionNotFound) {
			return err
		}

		if err := tx.InsertWagerTransaction(ctx, newTransaction); err != nil {
			return err
		}
		result = submitResultFromTransaction(newTransaction, false)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrIdempotencyKeyConflict) ||
			errors.Is(err, ErrExternalTransactionConflict) ||
			errors.Is(err, ErrWagerTransactionAlreadyExists) {
			return useCase.resolveConcurrentSubmission(ctx, command, err)
		}
		return SubmitWagerTransactionResult{}, err
	}
	return result, nil
}

func (useCase *SubmitWagerTransactionUseCase) resolveConcurrentSubmission(
	ctx context.Context,
	command SubmitWagerTransactionCommand,
	originalErr error,
) (SubmitWagerTransactionResult, error) {
	var result SubmitWagerTransactionResult
	err := useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		existing, err := tx.FindWagerTransactionByIdempotencyKeyForUpdate(
			ctx,
			command.ProviderID,
			command.IdempotencyKey,
		)
		if err == nil {
			if existing.PayloadHash() != command.PayloadHash {
				return ErrIdempotencyKeyConflict
			}
			result = submitResultFromTransaction(existing, true)
			return nil
		}
		if !errors.Is(err, ErrWagerTransactionNotFound) {
			return err
		}

		_, err = tx.FindWagerTransactionByExternalIDForUpdate(
			ctx,
			command.ProviderID,
			command.ExternalTransactionID,
		)
		if err == nil {
			return ErrExternalTransactionConflict
		}
		if !errors.Is(err, ErrWagerTransactionNotFound) {
			return err
		}
		return originalErr
	})
	if err != nil {
		return SubmitWagerTransactionResult{}, err
	}
	return result, nil
}

func submitResultFromTransaction(
	transaction domain.WagerTransaction,
	idempotentReplay bool,
) SubmitWagerTransactionResult {
	balance, hasBalance := transaction.ResultBalance()
	return SubmitWagerTransactionResult{
		TransactionID:    transaction.ID(),
		Status:           transaction.Status(),
		Balance:          balance,
		HasBalance:       hasBalance,
		FailureCode:      transaction.FailureCode(),
		IdempotentReplay: idempotentReplay,
	}
}

package application

import (
	"context"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

type ProcessLossCommand struct {
	TransactionID  string
	OutcomeEventID string
	CorrelationID  string
	CausationID    string
	ProcessedAt    time.Time
}

type ProcessLossResult = ProcessWagerResult

type ProcessLossUseCase struct {
	unitOfWork WageringUnitOfWork
}

func NewProcessLossUseCase(unitOfWork WageringUnitOfWork) (*ProcessLossUseCase, error) {
	if unitOfWork == nil {
		return nil, ErrNilWageringUnitOfWork
	}
	return &ProcessLossUseCase{unitOfWork: unitOfWork}, nil
}

func (useCase *ProcessLossUseCase) Execute(
	ctx context.Context,
	command ProcessLossCommand,
) (ProcessLossResult, error) {
	if command.TransactionID == "" ||
		command.OutcomeEventID == "" ||
		command.CorrelationID == "" ||
		command.ProcessedAt.IsZero() {
		return ProcessLossResult{}, ErrInvalidProcessWagerCommand
	}

	var result ProcessLossResult
	err := useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		transaction, wallet, err := loadPendingWagerAndWallet(
			ctx,
			tx,
			command.TransactionID,
			domain.WagerTransactionKindLoss,
		)
		if err != nil {
			return err
		}
		if err := transaction.MarkProcessed(wallet.Balance(), command.ProcessedAt); err != nil {
			return err
		}

		if err := tx.SaveWagerTransaction(ctx, transaction); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, newWagerTransactionProcessedEvent(
			command.OutcomeEventID,
			command.CorrelationID,
			command.CausationID,
			command.ProcessedAt,
			transaction,
			wallet.Balance(),
		)); err != nil {
			return err
		}

		result = ProcessLossResult{
			Status:        transaction.Status(),
			Balance:       wallet.Balance(),
			WalletVersion: wallet.Version(),
		}
		return nil
	})
	if err != nil {
		return ProcessLossResult{}, err
	}

	return result, nil
}

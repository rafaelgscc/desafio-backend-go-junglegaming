package application

import (
	"context"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

type ProcessWinCommand struct {
	TransactionID         string
	LedgerEntryID         string
	OutcomeEventID        string
	BalanceChangedEventID string
	CorrelationID         string
	CausationID           string
	ProcessedAt           time.Time
}

type ProcessWinResult = ProcessWagerResult

type ProcessWinUseCase struct {
	unitOfWork WageringUnitOfWork
}

func NewProcessWinUseCase(unitOfWork WageringUnitOfWork) (*ProcessWinUseCase, error) {
	if unitOfWork == nil {
		return nil, ErrNilWageringUnitOfWork
	}
	return &ProcessWinUseCase{unitOfWork: unitOfWork}, nil
}

func (useCase *ProcessWinUseCase) Execute(
	ctx context.Context,
	command ProcessWinCommand,
) (ProcessWinResult, error) {
	if command.TransactionID == "" ||
		command.LedgerEntryID == "" ||
		command.OutcomeEventID == "" ||
		command.BalanceChangedEventID == "" ||
		command.CorrelationID == "" ||
		command.ProcessedAt.IsZero() {
		return ProcessWinResult{}, ErrInvalidProcessWagerCommand
	}

	var result ProcessWinResult
	err := useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		transaction, wallet, err := loadPendingWagerAndWallet(
			ctx,
			tx,
			command.TransactionID,
			domain.WagerTransactionKindWin,
		)
		if err != nil {
			return err
		}

		balanceBefore := wallet.Balance()
		if err := wallet.Credit(transaction.Money(), command.ProcessedAt); err != nil {
			return err
		}

		entry, err := domain.NewWalletLedgerEntry(
			command.LedgerEntryID,
			wallet.ID(),
			transaction.ID(),
			domain.LedgerDirectionCredit,
			transaction.Money(),
			balanceBefore,
			wallet.Balance(),
			command.ProcessedAt,
		)
		if err != nil {
			return err
		}
		if err := transaction.MarkProcessed(wallet.Balance(), command.ProcessedAt); err != nil {
			return err
		}

		if err := tx.SaveWallet(ctx, wallet); err != nil {
			return err
		}
		if err := tx.AppendWalletLedgerEntry(ctx, entry); err != nil {
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
		if err := tx.AppendOutboxEvent(ctx, newWalletBalanceChangedEvent(
			command.BalanceChangedEventID,
			command.CorrelationID,
			command.CausationID,
			command.ProcessedAt,
			entry,
			wallet.Version(),
		)); err != nil {
			return err
		}

		result = ProcessWinResult{
			Status:        transaction.Status(),
			Balance:       wallet.Balance(),
			WalletVersion: wallet.Version(),
		}
		return nil
	})
	if err != nil {
		return ProcessWinResult{}, err
	}

	return result, nil
}

package application

import (
	"context"
	"errors"
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
	InboxCompletion       *InboxCompletion
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
		if transaction.ReferenceExternalTransactionID() != "" {
			reference, err := tx.FindWagerTransactionByExternalIDForUpdate(
				ctx,
				transaction.ProviderID(),
				transaction.ReferenceExternalTransactionID(),
			)
			if err != nil {
				if errors.Is(err, ErrWagerTransactionNotFound) {
					return rejectWin(
						ctx, tx, command, &transaction, wallet,
						domain.WagerTransactionFailureCodeReferenceNotFound,
						&result,
					)
				}
				return err
			}
			if reference.Status() != domain.WagerTransactionStatusProcessed {
				return rejectWin(
					ctx, tx, command, &transaction, wallet,
					domain.WagerTransactionFailureCodeReferenceNotProcessable,
					&result,
				)
			}
			if !validWinReference(transaction, reference) {
				return rejectWin(
					ctx, tx, command, &transaction, wallet,
					domain.WagerTransactionFailureCodeReferenceMismatch,
					&result,
				)
			}
			if err := transaction.ResolveReference(reference.ID(), command.ProcessedAt); err != nil {
				return err
			}
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
		if err := completeInboxIfRequested(ctx, tx, command.InboxCompletion); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ProcessWinResult{}, err
	}

	return result, nil
}

func rejectWin(
	ctx context.Context,
	tx WageringTransaction,
	command ProcessWinCommand,
	transaction *domain.WagerTransaction,
	wallet domain.Wallet,
	failureCode domain.WagerTransactionFailureCode,
	result *ProcessWinResult,
) error {
	if err := transaction.MarkRejected(failureCode, command.ProcessedAt); err != nil {
		return err
	}
	if err := tx.SaveWagerTransaction(ctx, *transaction); err != nil {
		return err
	}
	if err := tx.AppendOutboxEvent(ctx, newWagerTransactionRejectedEvent(
		command.OutcomeEventID,
		command.CorrelationID,
		command.CausationID,
		command.ProcessedAt,
		*transaction,
	)); err != nil {
		return err
	}
	*result = ProcessWinResult{
		Status:        transaction.Status(),
		Balance:       wallet.Balance(),
		WalletVersion: wallet.Version(),
		FailureCode:   transaction.FailureCode(),
	}
	if err := completeInboxIfRequested(ctx, tx, command.InboxCompletion); err != nil {
		return err
	}
	return nil
}

func validWinReference(
	transaction domain.WagerTransaction,
	reference domain.WagerTransaction,
) bool {
	return reference.Kind() == domain.WagerTransactionKindBet &&
		reference.ProviderID() == transaction.ProviderID() &&
		reference.PlayerID() == transaction.PlayerID() &&
		reference.WalletID() == transaction.WalletID() &&
		reference.RoundID() == transaction.RoundID()
}

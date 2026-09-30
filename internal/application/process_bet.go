package application

import (
	"context"
	"errors"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrNilWageringUnitOfWork          = errors.New("wagering unit of work is required")
	ErrInvalidProcessBetCommand       = errors.New("invalid process bet command")
	ErrUnexpectedWagerTransactionKind = errors.New("unexpected wager transaction kind")
	ErrWagerWalletMismatch            = errors.New("wager transaction does not belong to wallet")
	ErrWagerPlayerMismatch            = errors.New("wager transaction does not belong to wallet player")
)

type ProcessBetCommand struct {
	TransactionID string
	LedgerEntryID string
	ProcessedAt   time.Time
}

type ProcessBetResult struct {
	Status        domain.WagerTransactionStatus
	Balance       domain.Money
	WalletVersion int64
}

type ProcessBetUseCase struct {
	unitOfWork WageringUnitOfWork
}

func NewProcessBetUseCase(unitOfWork WageringUnitOfWork) (*ProcessBetUseCase, error) {
	if unitOfWork == nil {
		return nil, ErrNilWageringUnitOfWork
	}
	return &ProcessBetUseCase{unitOfWork: unitOfWork}, nil
}

func (useCase *ProcessBetUseCase) Execute(
	ctx context.Context,
	command ProcessBetCommand,
) (ProcessBetResult, error) {
	if command.TransactionID == "" ||
		command.LedgerEntryID == "" ||
		command.ProcessedAt.IsZero() {
		return ProcessBetResult{}, ErrInvalidProcessBetCommand
	}

	var result ProcessBetResult
	err := useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		transaction, err := tx.FindWagerTransactionForUpdate(ctx, command.TransactionID)
		if err != nil {
			return err
		}
		if transaction.Kind() != domain.WagerTransactionKindBet {
			return ErrUnexpectedWagerTransactionKind
		}
		if transaction.Status() != domain.WagerTransactionStatusPending {
			return domain.ErrInvalidWagerTransactionTransition
		}

		wallet, err := tx.FindWalletForUpdate(ctx, transaction.WalletID())
		if err != nil {
			return err
		}
		if wallet.ID() != transaction.WalletID() {
			return ErrWagerWalletMismatch
		}
		if wallet.PlayerID() != transaction.PlayerID() {
			return ErrWagerPlayerMismatch
		}

		balanceBefore := wallet.Balance()
		if err := wallet.Debit(transaction.Money(), command.ProcessedAt); err != nil {
			if !errors.Is(err, domain.ErrInsufficientFunds) {
				return err
			}

			if err := transaction.MarkRejected(
				domain.WagerTransactionFailureCodeInsufficientFunds,
				command.ProcessedAt,
			); err != nil {
				return err
			}
			if err := tx.SaveWagerTransaction(ctx, transaction); err != nil {
				return err
			}

			result = ProcessBetResult{
				Status:        transaction.Status(),
				Balance:       wallet.Balance(),
				WalletVersion: wallet.Version(),
			}
			return nil
		}

		entry, err := domain.NewWalletLedgerEntry(
			command.LedgerEntryID,
			wallet.ID(),
			transaction.ID(),
			domain.LedgerDirectionDebit,
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

		result = ProcessBetResult{
			Status:        transaction.Status(),
			Balance:       wallet.Balance(),
			WalletVersion: wallet.Version(),
		}
		return nil
	})
	if err != nil {
		return ProcessBetResult{}, err
	}

	return result, nil
}

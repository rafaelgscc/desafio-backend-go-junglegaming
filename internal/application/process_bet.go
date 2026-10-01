package application

import (
	"context"
	"errors"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrNilWageringUnitOfWork          = errors.New("wagering unit of work is required")
	ErrInvalidProcessWagerCommand     = errors.New("invalid process wager command")
	ErrInvalidProcessBetCommand       = ErrInvalidProcessWagerCommand
	ErrUnexpectedWagerTransactionKind = errors.New("unexpected wager transaction kind")
	ErrWagerWalletMismatch            = errors.New("wager transaction does not belong to wallet")
	ErrWagerPlayerMismatch            = errors.New("wager transaction does not belong to wallet player")
)

type ProcessBetCommand struct {
	TransactionID         string
	LedgerEntryID         string
	OutcomeEventID        string
	BalanceChangedEventID string
	CorrelationID         string
	CausationID           string
	ProcessedAt           time.Time
}

type ProcessWagerResult struct {
	Status        domain.WagerTransactionStatus
	Balance       domain.Money
	WalletVersion int64
	FailureCode   domain.WagerTransactionFailureCode
}

type ProcessBetResult = ProcessWagerResult

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
		command.OutcomeEventID == "" ||
		command.BalanceChangedEventID == "" ||
		command.CorrelationID == "" ||
		command.ProcessedAt.IsZero() {
		return ProcessBetResult{}, ErrInvalidProcessBetCommand
	}

	var result ProcessBetResult
	err := useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		transaction, wallet, err := loadPendingWagerAndWallet(
			ctx,
			tx,
			command.TransactionID,
			domain.WagerTransactionKindBet,
		)
		if err != nil {
			return err
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
			if err := tx.AppendOutboxEvent(ctx, newWagerTransactionRejectedEvent(
				command.OutcomeEventID,
				command.CorrelationID,
				command.CausationID,
				command.ProcessedAt,
				transaction,
			)); err != nil {
				return err
			}

			result = ProcessBetResult{
				Status:        transaction.Status(),
				Balance:       wallet.Balance(),
				WalletVersion: wallet.Version(),
				FailureCode:   transaction.FailureCode(),
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

func newWagerTransactionProcessedEvent(
	eventID string,
	correlationID string,
	causationID string,
	occurredAt time.Time,
	transaction domain.WagerTransaction,
	balance domain.Money,
) IntegrationEvent {
	return IntegrationEvent{
		EventID:       eventID,
		EventType:     IntegrationEventTypeWagerTransactionProcessed,
		AggregateID:   transaction.ID(),
		CorrelationID: correlationID,
		CausationID:   causationID,
		OccurredAt:    occurredAt.UTC(),
		Version:       IntegrationEventVersion,
		Data: WagerTransactionProcessedData{
			TransactionID:         transaction.ID(),
			ProviderID:            transaction.ProviderID(),
			ExternalTransactionID: transaction.ExternalTransactionID(),
			Kind:                  transaction.Kind(),
			Money:                 transaction.Money(),
			Balance:               balance,
		},
	}
}

func newWagerTransactionRejectedEvent(
	eventID string,
	correlationID string,
	causationID string,
	occurredAt time.Time,
	transaction domain.WagerTransaction,
) IntegrationEvent {
	return IntegrationEvent{
		EventID:       eventID,
		EventType:     IntegrationEventTypeWagerTransactionRejected,
		AggregateID:   transaction.ID(),
		CorrelationID: correlationID,
		CausationID:   causationID,
		OccurredAt:    occurredAt.UTC(),
		Version:       IntegrationEventVersion,
		Data: WagerTransactionRejectedData{
			TransactionID:         transaction.ID(),
			ProviderID:            transaction.ProviderID(),
			ExternalTransactionID: transaction.ExternalTransactionID(),
			Kind:                  transaction.Kind(),
			Money:                 transaction.Money(),
			FailureCode:           transaction.FailureCode(),
		},
	}
}

func newWalletBalanceChangedEvent(
	eventID string,
	correlationID string,
	causationID string,
	occurredAt time.Time,
	entry domain.WalletLedgerEntry,
	walletVersion int64,
) IntegrationEvent {
	return IntegrationEvent{
		EventID:       eventID,
		EventType:     IntegrationEventTypeWalletBalanceChanged,
		AggregateID:   entry.WalletID(),
		CorrelationID: correlationID,
		CausationID:   causationID,
		OccurredAt:    occurredAt.UTC(),
		Version:       IntegrationEventVersion,
		Data: WalletBalanceChangedData{
			WalletID:      entry.WalletID(),
			TransactionID: entry.TransactionID(),
			Direction:     entry.Direction(),
			Money:         entry.Money(),
			BalanceBefore: entry.BalanceBefore(),
			BalanceAfter:  entry.BalanceAfter(),
			WalletVersion: walletVersion,
		},
	}
}

func loadPendingWagerAndWallet(
	ctx context.Context,
	tx WageringTransaction,
	transactionID string,
	expectedKind domain.WagerTransactionKind,
) (domain.WagerTransaction, domain.Wallet, error) {
	transaction, err := tx.FindWagerTransactionForUpdate(ctx, transactionID)
	if err != nil {
		return domain.WagerTransaction{}, domain.Wallet{}, err
	}
	if transaction.Kind() != expectedKind {
		return domain.WagerTransaction{}, domain.Wallet{}, ErrUnexpectedWagerTransactionKind
	}
	if transaction.Status() != domain.WagerTransactionStatusPending {
		return domain.WagerTransaction{}, domain.Wallet{}, domain.ErrInvalidWagerTransactionTransition
	}

	wallet, err := tx.FindWalletForUpdate(ctx, transaction.WalletID())
	if err != nil {
		return domain.WagerTransaction{}, domain.Wallet{}, err
	}
	if wallet.ID() != transaction.WalletID() {
		return domain.WagerTransaction{}, domain.Wallet{}, ErrWagerWalletMismatch
	}
	if wallet.PlayerID() != transaction.PlayerID() {
		return domain.WagerTransaction{}, domain.Wallet{}, ErrWagerPlayerMismatch
	}

	return transaction, wallet, nil
}

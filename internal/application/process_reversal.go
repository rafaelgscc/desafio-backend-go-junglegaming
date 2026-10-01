package application

import (
	"context"
	"errors"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrWagerTransactionNotFound      = errors.New("wager transaction not found")
	ErrInvalidProcessReversalCommand = errors.New("invalid process reversal command")
	ErrInvalidReversalReferenceKind  = errors.New("invalid reversal reference kind")
)

type ProcessReversalCommand struct {
	TransactionID          string
	LedgerEntryID          string
	OutcomeEventID         string
	BalanceChangedEventID  string
	CorrelationID          string
	CausationID            string
	ProcessedAt            time.Time
	NextReferenceAttemptAt time.Time
}

type ProcessReversalResult = ProcessWagerResult

type ProcessReversalUseCase struct {
	unitOfWork   WageringUnitOfWork
	expectedKind domain.WagerTransactionKind
}

func NewProcessRefundUseCase(unitOfWork WageringUnitOfWork) (*ProcessReversalUseCase, error) {
	return newProcessReversalUseCase(unitOfWork, domain.WagerTransactionKindRefund)
}

func NewProcessRollbackUseCase(unitOfWork WageringUnitOfWork) (*ProcessReversalUseCase, error) {
	return newProcessReversalUseCase(unitOfWork, domain.WagerTransactionKindRollback)
}

func newProcessReversalUseCase(
	unitOfWork WageringUnitOfWork,
	expectedKind domain.WagerTransactionKind,
) (*ProcessReversalUseCase, error) {
	if unitOfWork == nil {
		return nil, ErrNilWageringUnitOfWork
	}
	return &ProcessReversalUseCase{unitOfWork: unitOfWork, expectedKind: expectedKind}, nil
}

func (useCase *ProcessReversalUseCase) Execute(
	ctx context.Context,
	command ProcessReversalCommand,
) (ProcessReversalResult, error) {
	if command.TransactionID == "" || command.LedgerEntryID == "" ||
		command.OutcomeEventID == "" || command.BalanceChangedEventID == "" ||
		command.CorrelationID == "" || command.ProcessedAt.IsZero() ||
		command.NextReferenceAttemptAt.IsZero() ||
		!command.NextReferenceAttemptAt.After(command.ProcessedAt) {
		return ProcessReversalResult{}, ErrInvalidProcessReversalCommand
	}

	var result ProcessReversalResult
	err := useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		transaction, wallet, err := loadReversalAndWallet(
			ctx, tx, command.TransactionID, useCase.expectedKind,
		)
		if err != nil {
			return err
		}

		reference, err := tx.FindWagerTransactionByExternalIDForUpdate(
			ctx,
			transaction.ProviderID(),
			transaction.ReferenceExternalTransactionID(),
		)
		if err != nil {
			if errors.Is(err, ErrWagerTransactionNotFound) {
				return persistPendingReference(ctx, tx, command, &transaction, wallet, &result)
			}
			return err
		}

		if reference.Status() == domain.WagerTransactionStatusPending ||
			reference.Status() == domain.WagerTransactionStatusPendingReference {
			return persistPendingReference(ctx, tx, command, &transaction, wallet, &result)
		}
		if reference.Status() != domain.WagerTransactionStatusProcessed {
			return persistRejectedReversal(
				ctx, tx, command, &transaction, wallet,
				domain.WagerTransactionFailureCodeReferenceNotProcessable,
				&result,
			)
		}

		if !validReversalReference(transaction, reference) {
			return persistRejectedReversal(
				ctx, tx, command, &transaction, wallet,
				domain.WagerTransactionFailureCodeReferenceMismatch,
				&result,
			)
		}
		if !referenceKindAllowed(transaction.Kind(), reference.Kind()) {
			return persistRejectedReversal(
				ctx, tx, command, &transaction, wallet,
				domain.WagerTransactionFailureCodeReferenceMismatch,
				&result,
			)
		}

		duplicate, err := tx.HasProcessedReversal(ctx, reference.ID(), transaction.Kind())
		if err != nil {
			return err
		}
		if duplicate {
			return persistRejectedReversal(
				ctx, tx, command, &transaction, wallet,
				domain.WagerTransactionFailureCodeDuplicateReversal,
				&result,
			)
		}

		if err := transaction.ResolveReference(reference.ID(), command.ProcessedAt); err != nil {
			return err
		}
		return applyReversal(ctx, tx, command, &transaction, wallet, reference, &result)
	})
	if err != nil {
		return ProcessReversalResult{}, err
	}
	return result, nil
}

func loadReversalAndWallet(
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
	if transaction.Status() != domain.WagerTransactionStatusPending &&
		transaction.Status() != domain.WagerTransactionStatusPendingReference {
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

func persistPendingReference(
	ctx context.Context,
	tx WageringTransaction,
	command ProcessReversalCommand,
	transaction *domain.WagerTransaction,
	wallet domain.Wallet,
	result *ProcessReversalResult,
) error {
	var err error
	if transaction.Status() == domain.WagerTransactionStatusPending {
		err = transaction.MarkPendingReference(command.NextReferenceAttemptAt, command.ProcessedAt)
	} else {
		err = transaction.RescheduleReference(command.NextReferenceAttemptAt, command.ProcessedAt)
	}
	if err != nil {
		return err
	}
	if err := tx.SaveWagerTransaction(ctx, *transaction); err != nil {
		return err
	}
	if err := tx.AppendOutboxEvent(ctx, newWagerTransactionPendingReferenceEvent(
		command, *transaction,
	)); err != nil {
		return err
	}
	*result = ProcessReversalResult{
		Status: transaction.Status(), Balance: wallet.Balance(), WalletVersion: wallet.Version(),
		FailureCode: transaction.FailureCode(),
	}
	return nil
}

func persistRejectedReversal(
	ctx context.Context,
	tx WageringTransaction,
	command ProcessReversalCommand,
	transaction *domain.WagerTransaction,
	wallet domain.Wallet,
	failureCode domain.WagerTransactionFailureCode,
	result *ProcessReversalResult,
) error {
	if err := transaction.MarkRejected(failureCode, command.ProcessedAt); err != nil {
		return err
	}
	if err := tx.SaveWagerTransaction(ctx, *transaction); err != nil {
		return err
	}
	if err := tx.AppendOutboxEvent(ctx, newWagerTransactionRejectedEvent(
		command.OutcomeEventID, command.CorrelationID, command.CausationID,
		command.ProcessedAt, *transaction,
	)); err != nil {
		return err
	}
	*result = ProcessReversalResult{
		Status: transaction.Status(), Balance: wallet.Balance(), WalletVersion: wallet.Version(),
		FailureCode: transaction.FailureCode(),
	}
	return nil
}

func applyReversal(
	ctx context.Context,
	tx WageringTransaction,
	command ProcessReversalCommand,
	transaction *domain.WagerTransaction,
	wallet domain.Wallet,
	reference domain.WagerTransaction,
	result *ProcessReversalResult,
) error {
	balanceBefore := wallet.Balance()
	direction := reversalDirection(transaction.Kind(), reference.Kind())
	var err error
	if direction == domain.LedgerDirectionCredit {
		err = wallet.Credit(transaction.Money(), command.ProcessedAt)
	} else {
		err = wallet.Debit(transaction.Money(), command.ProcessedAt)
	}
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientFunds) {
			return persistRejectedReversal(
				ctx, tx, command, transaction, wallet,
				domain.WagerTransactionFailureCodeReversalInsufficientFunds,
				result,
			)
		}
		return err
	}

	entry, err := domain.NewWalletLedgerEntry(
		command.LedgerEntryID, wallet.ID(), transaction.ID(), direction,
		transaction.Money(), balanceBefore, wallet.Balance(), command.ProcessedAt,
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
	if err := tx.SaveWagerTransaction(ctx, *transaction); err != nil {
		return err
	}
	if err := tx.AppendOutboxEvent(ctx, newWagerTransactionProcessedEvent(
		command.OutcomeEventID, command.CorrelationID, command.CausationID,
		command.ProcessedAt, *transaction, wallet.Balance(),
	)); err != nil {
		return err
	}
	if err := tx.AppendOutboxEvent(ctx, newWalletBalanceChangedEvent(
		command.BalanceChangedEventID, command.CorrelationID, command.CausationID,
		command.ProcessedAt, entry, wallet.Version(),
	)); err != nil {
		return err
	}

	*result = ProcessReversalResult{
		Status: transaction.Status(), Balance: wallet.Balance(), WalletVersion: wallet.Version(),
		FailureCode: transaction.FailureCode(),
	}
	return nil
}

func validReversalReference(
	transaction domain.WagerTransaction,
	reference domain.WagerTransaction,
) bool {
	if reference.ProviderID() != transaction.ProviderID() ||
		reference.PlayerID() != transaction.PlayerID() ||
		reference.WalletID() != transaction.WalletID() ||
		reference.RoundID() != transaction.RoundID() {
		return false
	}
	comparison, err := reference.Money().Compare(transaction.Money())
	return err == nil && comparison == 0
}

func referenceKindAllowed(reversalKind, referenceKind domain.WagerTransactionKind) bool {
	if reversalKind == domain.WagerTransactionKindRefund {
		return referenceKind == domain.WagerTransactionKindBet
	}
	return referenceKind == domain.WagerTransactionKindBet ||
		referenceKind == domain.WagerTransactionKindWin ||
		referenceKind == domain.WagerTransactionKindRefund
}

func reversalDirection(
	reversalKind domain.WagerTransactionKind,
	referenceKind domain.WagerTransactionKind,
) domain.LedgerDirection {
	if reversalKind == domain.WagerTransactionKindRefund ||
		referenceKind == domain.WagerTransactionKindBet {
		return domain.LedgerDirectionCredit
	}
	return domain.LedgerDirectionDebit
}

func newWagerTransactionPendingReferenceEvent(
	command ProcessReversalCommand,
	transaction domain.WagerTransaction,
) IntegrationEvent {
	nextAttemptAt, _ := transaction.NextReferenceAttemptAt()
	return IntegrationEvent{
		EventID:       command.OutcomeEventID,
		EventType:     IntegrationEventTypeWagerTransactionPendingReference,
		AggregateID:   transaction.ID(),
		CorrelationID: command.CorrelationID,
		CausationID:   command.CausationID,
		OccurredAt:    command.ProcessedAt.UTC(),
		Version:       IntegrationEventVersion,
		Data: WagerTransactionPendingReferenceData{
			TransactionID:                  transaction.ID(),
			ProviderID:                     transaction.ProviderID(),
			ExternalTransactionID:          transaction.ExternalTransactionID(),
			ReferenceExternalTransactionID: transaction.ReferenceExternalTransactionID(),
			ReferenceAttempts:              transaction.ReferenceAttempts(),
			NextReferenceAttemptAt:         nextAttemptAt,
		},
	}
}

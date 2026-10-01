package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestProcessBetDebitsWalletAndPersistsFinancialResult(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processBetFixtures(t, "100.00", "25.00")
	unitOfWork := &fakeWageringUnitOfWork{wallet: wallet, transaction: transaction}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessBetCommand{
		TransactionID:         transaction.ID(),
		LedgerEntryID:         "ledger-entry-1",
		OutcomeEventID:        "event-wager-processed-1",
		BalanceChangedEventID: "event-wallet-balance-1",
		CorrelationID:         "correlation-1",
		CausationID:           "request-1",
		ProcessedAt:           now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if result.Status != domain.WagerTransactionStatusProcessed {
		t.Errorf("result.Status = %q, want %q", result.Status, domain.WagerTransactionStatusProcessed)
	}
	if got := result.Balance.Amount(); got != "75.00" {
		t.Errorf("result.Balance = %q, want 75.00", got)
	}
	if result.WalletVersion != 2 {
		t.Errorf("result.WalletVersion = %d, want 2", result.WalletVersion)
	}

	if got := unitOfWork.wallet.Balance().Amount(); got != "75.00" {
		t.Errorf("persisted wallet balance = %q, want 75.00", got)
	}
	if unitOfWork.transaction.Status() != domain.WagerTransactionStatusProcessed {
		t.Errorf("persisted transaction status = %q, want PROCESSED", unitOfWork.transaction.Status())
	}
	if unitOfWork.ledgerEntry == nil {
		t.Fatal("no ledger entry was persisted")
	}
	if unitOfWork.ledgerEntry.Direction() != domain.LedgerDirectionDebit {
		t.Errorf("ledger direction = %q, want DEBIT", unitOfWork.ledgerEntry.Direction())
	}
	if got := unitOfWork.ledgerEntry.BalanceBefore().Amount(); got != "100.00" {
		t.Errorf("ledger balance before = %q, want 100.00", got)
	}
	if got := unitOfWork.ledgerEntry.BalanceAfter().Amount(); got != "75.00" {
		t.Errorf("ledger balance after = %q, want 75.00", got)
	}
	if len(unitOfWork.outboxEvents) != 2 {
		t.Fatalf("outbox event count = %d, want 2", len(unitOfWork.outboxEvents))
	}
	if unitOfWork.outboxEvents[0].EventType != IntegrationEventTypeWagerTransactionProcessed {
		t.Errorf("first outbox event type = %q, want %q", unitOfWork.outboxEvents[0].EventType, IntegrationEventTypeWagerTransactionProcessed)
	}
	if unitOfWork.outboxEvents[1].EventType != IntegrationEventTypeWalletBalanceChanged {
		t.Errorf("second outbox event type = %q, want %q", unitOfWork.outboxEvents[1].EventType, IntegrationEventTypeWalletBalanceChanged)
	}
	balanceData, ok := unitOfWork.outboxEvents[1].Data.(WalletBalanceChangedData)
	if !ok {
		t.Fatalf("wallet event data type = %T, want WalletBalanceChangedData", unitOfWork.outboxEvents[1].Data)
	}
	if balanceData.WalletVersion != 2 || balanceData.BalanceAfter.Amount() != "75.00" {
		t.Errorf("wallet event data = version %d balance %s, want version 2 balance 75.00", balanceData.WalletVersion, balanceData.BalanceAfter.Amount())
	}
}

func TestProcessBetRejectsInsufficientFundsWithoutChangingWalletOrLedger(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processBetFixtures(t, "20.00", "25.00")
	unitOfWork := &fakeWageringUnitOfWork{wallet: wallet, transaction: transaction}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	result, err := useCase.Execute(context.Background(), ProcessBetCommand{
		TransactionID:         transaction.ID(),
		LedgerEntryID:         "ledger-entry-1",
		OutcomeEventID:        "event-wager-rejected-1",
		BalanceChangedEventID: "event-wallet-balance-unused",
		CorrelationID:         "correlation-1",
		CausationID:           "request-1",
		ProcessedAt:           now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if result.Status != domain.WagerTransactionStatusRejected {
		t.Errorf("result.Status = %q, want REJECTED", result.Status)
	}
	if got := result.Balance.Amount(); got != "20.00" {
		t.Errorf("result.Balance = %q, want 20.00", got)
	}
	if unitOfWork.wallet.Version() != 1 || unitOfWork.wallet.Balance().Amount() != "20.00" {
		t.Errorf("wallet changed after rejection: version=%d balance=%s", unitOfWork.wallet.Version(), unitOfWork.wallet.Balance().Amount())
	}
	if unitOfWork.ledgerEntry != nil {
		t.Error("ledger entry was persisted for rejected bet")
	}
	if unitOfWork.transaction.Status() != domain.WagerTransactionStatusRejected {
		t.Errorf("transaction status = %q, want REJECTED", unitOfWork.transaction.Status())
	}
	if unitOfWork.transaction.FailureCode() != domain.WagerTransactionFailureCodeInsufficientFunds {
		t.Errorf("failure code = %q, want %q", unitOfWork.transaction.FailureCode(), domain.WagerTransactionFailureCodeInsufficientFunds)
	}
	if len(unitOfWork.outboxEvents) != 1 || unitOfWork.outboxEvents[0].EventType != IntegrationEventTypeWagerTransactionRejected {
		t.Fatalf("outbox events = %#v, want one WagerTransactionRejected", unitOfWork.outboxEvents)
	}
	rejectedData, ok := unitOfWork.outboxEvents[0].Data.(WagerTransactionRejectedData)
	if !ok || rejectedData.FailureCode != domain.WagerTransactionFailureCodeInsufficientFunds {
		t.Errorf("rejected event data = %#v, want INSUFFICIENT_FUNDS", unitOfWork.outboxEvents[0].Data)
	}
}

func TestProcessBetRejectsInvalidCommandBeforeOpeningTransaction(t *testing.T) {
	t.Parallel()

	unitOfWork := &fakeWageringUnitOfWork{}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	_, err = useCase.Execute(context.Background(), ProcessBetCommand{})
	if !errors.Is(err, ErrInvalidProcessBetCommand) {
		t.Fatalf("Execute() error = %v, want ErrInvalidProcessBetCommand", err)
	}
	if unitOfWork.calls != 0 {
		t.Errorf("WithinTransaction() calls = %d, want 0", unitOfWork.calls)
	}
}

func TestProcessBetRollsBackEveryChangeWhenPersistenceFails(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processBetFixtures(t, "100.00", "25.00")
	persistenceErr := errors.New("append ledger failed")
	unitOfWork := &fakeWageringUnitOfWork{
		wallet:          wallet,
		transaction:     transaction,
		appendLedgerErr: persistenceErr,
	}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	_, err = useCase.Execute(context.Background(), ProcessBetCommand{
		TransactionID:         transaction.ID(),
		LedgerEntryID:         "ledger-entry-1",
		OutcomeEventID:        "event-wager-processed-1",
		BalanceChangedEventID: "event-wallet-balance-1",
		CorrelationID:         "correlation-1",
		ProcessedAt:           now.Add(time.Second),
	})
	if !errors.Is(err, persistenceErr) {
		t.Fatalf("Execute() error = %v, want persistence error", err)
	}
	if unitOfWork.wallet.Balance().Amount() != "100.00" || unitOfWork.wallet.Version() != 1 {
		t.Errorf("wallet was committed after rollback: balance=%s version=%d", unitOfWork.wallet.Balance().Amount(), unitOfWork.wallet.Version())
	}
	if unitOfWork.transaction.Status() != domain.WagerTransactionStatusPending {
		t.Errorf("transaction status was committed as %q, want PENDING", unitOfWork.transaction.Status())
	}
	if unitOfWork.ledgerEntry != nil {
		t.Error("ledger entry was committed after rollback")
	}
	if len(unitOfWork.outboxEvents) != 0 {
		t.Errorf("outbox events were committed after rollback: %d", len(unitOfWork.outboxEvents))
	}
}

func TestProcessBetRollsBackFinancialChangesWhenOutboxPersistenceFails(t *testing.T) {
	t.Parallel()

	wallet, transaction, now := processBetFixtures(t, "100.00", "25.00")
	outboxErr := errors.New("append outbox failed")
	unitOfWork := &fakeWageringUnitOfWork{
		wallet:          wallet,
		transaction:     transaction,
		appendOutboxErr: outboxErr,
	}
	useCase, err := NewProcessBetUseCase(unitOfWork)
	if err != nil {
		t.Fatalf("NewProcessBetUseCase() unexpected error: %v", err)
	}

	_, err = useCase.Execute(context.Background(), ProcessBetCommand{
		TransactionID:         transaction.ID(),
		LedgerEntryID:         "ledger-entry-1",
		OutcomeEventID:        "event-wager-processed-1",
		BalanceChangedEventID: "event-wallet-balance-1",
		CorrelationID:         "correlation-1",
		ProcessedAt:           now.Add(time.Second),
	})
	if !errors.Is(err, outboxErr) {
		t.Fatalf("Execute() error = %v, want outbox error", err)
	}
	if unitOfWork.wallet.Balance().Amount() != "100.00" || unitOfWork.wallet.Version() != 1 {
		t.Errorf("wallet was committed after rollback: balance=%s version=%d", unitOfWork.wallet.Balance().Amount(), unitOfWork.wallet.Version())
	}
	if unitOfWork.transaction.Status() != domain.WagerTransactionStatusPending {
		t.Errorf("transaction status was committed as %q, want PENDING", unitOfWork.transaction.Status())
	}
	if unitOfWork.ledgerEntry != nil || len(unitOfWork.outboxEvents) != 0 {
		t.Errorf("ledger/outbox was committed after rollback: ledger=%v events=%d", unitOfWork.ledgerEntry != nil, len(unitOfWork.outboxEvents))
	}
}

func processBetFixtures(
	t *testing.T,
	walletAmount string,
	betAmount string,
) (domain.Wallet, domain.WagerTransaction, time.Time) {
	t.Helper()
	return processWagerFixtures(
		t,
		domain.WagerTransactionKindBet,
		walletAmount,
		betAmount,
	)
}

func processWagerFixtures(
	t *testing.T,
	kind domain.WagerTransactionKind,
	walletAmount string,
	wagerAmount string,
) (domain.Wallet, domain.WagerTransaction, time.Time) {
	t.Helper()

	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	walletMoney, err := domain.NewMoney(walletAmount, "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected wallet setup error: %v", err)
	}
	wallet, err := domain.NewWallet("wallet-1", "player-1", walletMoney, now)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}
	wagerMoney, err := domain.NewMoney(wagerAmount, "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected wager setup error: %v", err)
	}
	transaction, err := domain.NewExternalWagerTransaction(domain.NewExternalWagerTransactionParams{
		ID:                    "wager-transaction-1",
		ExternalTransactionID: "provider-transaction-1",
		ProviderID:            "provider-a",
		IdempotencyKey:        "provider-a:provider-transaction-1",
		PayloadHash:           "sha256:payload",
		WalletID:              wallet.ID(),
		PlayerID:              wallet.PlayerID(),
		RoundID:               "round-1",
		GameID:                "game-1",
		Kind:                  kind,
		Money:                 wagerMoney,
		OccurredAt:            now,
	})
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}
	return wallet, transaction, now
}

type fakeWageringUnitOfWork struct {
	wallet                domain.Wallet
	transaction           domain.WagerTransaction
	ledgerEntry           *domain.WalletLedgerEntry
	calls                 int
	fail                  error
	appendLedgerErr       error
	appendOutboxErr       error
	outboxEvents          []IntegrationEvent
	referencedTransaction domain.WagerTransaction
	referenceErr          error
	hasProcessedReversal  bool
	idempotentTransaction domain.WagerTransaction
	idempotencyErr        error
	insertedTransaction   bool
	walletExists          bool
	insertedWallet        bool
}

func (unitOfWork *fakeWageringUnitOfWork) WithinTransaction(
	_ context.Context,
	fn func(WageringTransaction) error,
) error {
	unitOfWork.calls++
	if unitOfWork.fail != nil {
		return unitOfWork.fail
	}

	working := &fakeWageringTransaction{
		wallet:                unitOfWork.wallet,
		transaction:           unitOfWork.transaction,
		appendLedgerErr:       unitOfWork.appendLedgerErr,
		appendOutboxErr:       unitOfWork.appendOutboxErr,
		referencedTransaction: unitOfWork.referencedTransaction,
		referenceErr:          unitOfWork.referenceErr,
		hasProcessedReversal:  unitOfWork.hasProcessedReversal,
		idempotentTransaction: unitOfWork.idempotentTransaction,
		idempotencyErr:        unitOfWork.idempotencyErr,
		walletExists:          unitOfWork.walletExists,
	}
	if err := fn(working); err != nil {
		return err
	}

	unitOfWork.wallet = working.wallet
	unitOfWork.transaction = working.transaction
	unitOfWork.ledgerEntry = working.ledgerEntry
	unitOfWork.outboxEvents = working.outboxEvents
	unitOfWork.insertedTransaction = working.insertedTransaction
	unitOfWork.insertedWallet = working.insertedWallet
	return nil
}

type fakeWageringTransaction struct {
	wallet                domain.Wallet
	transaction           domain.WagerTransaction
	ledgerEntry           *domain.WalletLedgerEntry
	appendLedgerErr       error
	appendOutboxErr       error
	outboxEvents          []IntegrationEvent
	referencedTransaction domain.WagerTransaction
	referenceErr          error
	hasProcessedReversal  bool
	idempotentTransaction domain.WagerTransaction
	idempotencyErr        error
	insertedTransaction   bool
	walletExists          bool
	insertedWallet        bool
}

func (tx *fakeWageringTransaction) FindWagerTransactionForUpdate(
	_ context.Context,
	_ string,
) (domain.WagerTransaction, error) {
	return tx.transaction, nil
}

func (tx *fakeWageringTransaction) FindWalletForUpdate(
	_ context.Context,
	_ string,
) (domain.Wallet, error) {
	return tx.wallet, nil
}

func (tx *fakeWageringTransaction) WalletExistsForPlayerAndCurrency(
	_ context.Context,
	_ string,
	_ string,
) (bool, error) {
	return tx.walletExists, nil
}

func (tx *fakeWageringTransaction) FindWagerTransactionByExternalIDForUpdate(
	_ context.Context,
	_ string,
	_ string,
) (domain.WagerTransaction, error) {
	if tx.referenceErr != nil {
		return domain.WagerTransaction{}, tx.referenceErr
	}
	return tx.referencedTransaction, nil
}

func (tx *fakeWageringTransaction) FindWagerTransactionByIdempotencyKeyForUpdate(
	_ context.Context,
	_ string,
	_ string,
) (domain.WagerTransaction, error) {
	if tx.idempotencyErr != nil {
		return domain.WagerTransaction{}, tx.idempotencyErr
	}
	return tx.idempotentTransaction, nil
}

func (tx *fakeWageringTransaction) HasProcessedReversal(
	_ context.Context,
	_ string,
	_ domain.WagerTransactionKind,
) (bool, error) {
	return tx.hasProcessedReversal, nil
}

func (tx *fakeWageringTransaction) SaveWagerTransaction(
	_ context.Context,
	transaction domain.WagerTransaction,
) error {
	tx.transaction = transaction
	return nil
}

func (tx *fakeWageringTransaction) InsertWagerTransaction(
	_ context.Context,
	transaction domain.WagerTransaction,
) error {
	tx.transaction = transaction
	tx.insertedTransaction = true
	return nil
}

func (tx *fakeWageringTransaction) InsertWallet(
	_ context.Context,
	wallet domain.Wallet,
) error {
	tx.wallet = wallet
	tx.insertedWallet = true
	return nil
}

func (tx *fakeWageringTransaction) SaveWallet(_ context.Context, wallet domain.Wallet) error {
	tx.wallet = wallet
	return nil
}

func (tx *fakeWageringTransaction) AppendWalletLedgerEntry(
	_ context.Context,
	entry domain.WalletLedgerEntry,
) error {
	if tx.appendLedgerErr != nil {
		return tx.appendLedgerErr
	}
	tx.ledgerEntry = &entry
	return nil
}

func (tx *fakeWageringTransaction) AppendOutboxEvent(
	_ context.Context,
	event IntegrationEvent,
) error {
	if tx.appendOutboxErr != nil {
		return tx.appendOutboxErr
	}
	tx.outboxEvents = append(tx.outboxEvents, event)
	return nil
}

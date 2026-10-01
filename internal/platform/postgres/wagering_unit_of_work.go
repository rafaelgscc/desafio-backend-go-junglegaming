package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

var (
	ErrPostgresPoolRequired  = errors.New("PostgreSQL pool is required")
	ErrTransactionFnRequired = errors.New("transaction function is required")
)

type PostgresWageringUnitOfWork struct {
	pool *pgxpool.Pool
}

func NewPostgresWageringUnitOfWork(
	pool *pgxpool.Pool,
) (*PostgresWageringUnitOfWork, error) {
	if pool == nil {
		return nil, ErrPostgresPoolRequired
	}

	return &PostgresWageringUnitOfWork{pool: pool}, nil
}

func (unitOfWork *PostgresWageringUnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(application.WageringTransaction) error,
) error {
	if fn == nil {
		return ErrTransactionFnRequired
	}

	tx, err := unitOfWork.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin wagering transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	transaction, err := newPostgresWageringTransaction(tx)
	if err != nil {
		return err
	}

	if err := fn(transaction); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit wagering transaction: %w", err)
	}

	return nil
}

type postgresWageringTransaction struct {
	*WalletRepository
	*WagerTransactionRepository
	*WalletLedgerEntryRepository
	*OutboxRepository
}

var _ application.WageringUnitOfWork = (*PostgresWageringUnitOfWork)(nil)
var _ application.WageringTransaction = (*postgresWageringTransaction)(nil)

func newPostgresWageringTransaction(
	tx pgx.Tx,
) (*postgresWageringTransaction, error) {
	walletRepository, err := NewWalletRepository(tx)
	if err != nil {
		return nil, fmt.Errorf("create wallet repository: %w", err)
	}
	wagerTransactionRepository, err := NewWagerTransactionRepository(tx)
	if err != nil {
		return nil, fmt.Errorf("create wager transaction repository: %w", err)
	}
	walletLedgerEntryRepository, err := NewWalletLedgerEntryRepository(tx)
	if err != nil {
		return nil, fmt.Errorf("create wallet ledger entry repository: %w", err)
	}
	outboxRepository, err := NewOutboxRepository(tx)
	if err != nil {
		return nil, fmt.Errorf("create outbox repository: %w", err)
	}

	return &postgresWageringTransaction{
		WalletRepository:            walletRepository,
		WagerTransactionRepository:  wagerTransactionRepository,
		WalletLedgerEntryRepository: walletLedgerEntryRepository,
		OutboxRepository:            outboxRepository,
	}, nil
}

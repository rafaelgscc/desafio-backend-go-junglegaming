package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

type WalletLedgerEntryRepository struct {
	db DBTX
}

func NewWalletLedgerEntryRepository(db DBTX) (*WalletLedgerEntryRepository, error) {
	if db == nil {
		return nil, ErrDatabaseExecutorRequired
	}

	return &WalletLedgerEntryRepository{db: db}, nil
}

func (repository *WalletLedgerEntryRepository) AppendWalletLedgerEntry(
	ctx context.Context,
	entry domain.WalletLedgerEntry,
) error {
	_, err := repository.db.Exec(ctx, `
		INSERT INTO wallet_ledger_entries (
			id,
			wallet_id,
			transaction_id,
			direction,
			amount_in_cents,
			currency,
			balance_before_in_cents,
			balance_after_in_cents,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		entry.ID(),
		entry.WalletID(),
		entry.TransactionID(),
		string(entry.Direction()),
		entry.Money().AmountInCents(),
		entry.Money().Currency(),
		entry.BalanceBefore().AmountInCents(),
		entry.BalanceAfter().AmountInCents(),
		entry.CreatedAt(),
	)
	if isWalletLedgerEntryUniqueViolation(err) {
		return fmt.Errorf("%w: %v", application.ErrWalletLedgerEntryAlreadyExists, err)
	}
	if err != nil {
		return fmt.Errorf("append wallet ledger entry: %w", err)
	}

	return nil
}

func isWalletLedgerEntryUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

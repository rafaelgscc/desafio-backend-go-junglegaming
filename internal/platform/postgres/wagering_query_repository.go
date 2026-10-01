package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

type WageringQueryRepository struct {
	db           DBTX
	wallets      *WalletRepository
	transactions *WagerTransactionRepository
}

func NewWageringQueryRepository(db *pgxpool.Pool) (*WageringQueryRepository, error) {
	return newWageringQueryRepository(db)
}

func newWageringQueryRepository(db DBTX) (*WageringQueryRepository, error) {
	if db == nil {
		return nil, ErrDatabaseExecutorRequired
	}
	wallets, _ := NewWalletRepository(db)
	transactions, _ := NewWagerTransactionRepository(db)
	return &WageringQueryRepository{db: db, wallets: wallets, transactions: transactions}, nil
}

func (repository *WageringQueryRepository) FindWallet(
	ctx context.Context,
	walletID string,
) (domain.Wallet, error) {
	return repository.wallets.FindWallet(ctx, walletID)
}

func (repository *WageringQueryRepository) FindWagerTransactionForProvider(
	ctx context.Context,
	providerID string,
	transactionID string,
) (domain.WagerTransaction, error) {
	return repository.transactions.FindWagerTransactionForProvider(ctx, providerID, transactionID)
}

func (repository *WageringQueryRepository) FindWagerTransactionByExternalID(
	ctx context.Context,
	providerID string,
	externalTransactionID string,
) (domain.WagerTransaction, error) {
	return repository.transactions.FindWagerTransactionByExternalID(ctx, providerID, externalTransactionID)
}

func (repository *WageringQueryRepository) ListWalletLedger(
	ctx context.Context,
	walletID string,
	cursor *application.LedgerCursor,
	limit int,
) ([]domain.WalletLedgerEntry, error) {
	query := `
		SELECT id, wallet_id, transaction_id, direction, amount_in_cents, currency,
			balance_before_in_cents, balance_after_in_cents, created_at
		FROM wallet_ledger_entries
		WHERE wallet_id = $1
		ORDER BY created_at, id
		LIMIT $2`
	args := []any{walletID, limit}
	if cursor != nil {
		query = `
			SELECT id, wallet_id, transaction_id, direction, amount_in_cents, currency,
				balance_before_in_cents, balance_after_in_cents, created_at
			FROM wallet_ledger_entries
			WHERE wallet_id = $1 AND (created_at, id) > ($2, $3)
			ORDER BY created_at, id
			LIMIT $4`
		args = []any{walletID, cursor.CreatedAt.UTC(), cursor.EntryID, limit}
	}

	rows, err := repository.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list wallet ledger: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.WalletLedgerEntry, 0)
	for rows.Next() {
		entry, err := scanWalletLedgerEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wallet ledger: %w", err)
	}
	return entries, nil
}

func scanWalletLedgerEntry(row pgx.Row) (domain.WalletLedgerEntry, error) {
	var id, walletID, transactionID, direction, currency string
	var amount, balanceBefore, balanceAfter int64
	var createdAt time.Time
	if err := row.Scan(
		&id, &walletID, &transactionID, &direction, &amount, &currency,
		&balanceBefore, &balanceAfter, &createdAt,
	); err != nil {
		return domain.WalletLedgerEntry{}, fmt.Errorf("scan wallet ledger entry: %w", err)
	}
	money, err := moneyFromCents(amount, currency)
	if err != nil {
		return domain.WalletLedgerEntry{}, fmt.Errorf("restore ledger money: %w", err)
	}
	before, err := moneyFromCents(balanceBefore, currency)
	if err != nil {
		return domain.WalletLedgerEntry{}, fmt.Errorf("restore ledger balance before: %w", err)
	}
	after, err := moneyFromCents(balanceAfter, currency)
	if err != nil {
		return domain.WalletLedgerEntry{}, fmt.Errorf("restore ledger balance after: %w", err)
	}
	entry, err := domain.NewWalletLedgerEntry(
		id, walletID, transactionID, domain.LedgerDirection(direction),
		money, before, after, createdAt,
	)
	if err != nil {
		return domain.WalletLedgerEntry{}, fmt.Errorf("rehydrate wallet ledger entry: %w", err)
	}
	return entry, nil
}

func (repository *WageringQueryRepository) ReconcileWallet(
	ctx context.Context,
	walletID string,
) (application.ReconciliationResult, error) {
	var id, playerID, currency string
	var storedInCents, version, calculatedInCents, checkedEntries int64
	var createdAt, updatedAt time.Time
	err := repository.db.QueryRow(ctx, `
		SELECT w.id, w.player_id, w.currency, w.balance_in_cents, w.version,
			w.created_at, w.updated_at,
			COALESCE(SUM(CASE l.direction
				WHEN 'CREDIT' THEN l.amount_in_cents
				WHEN 'DEBIT' THEN -l.amount_in_cents
			END), 0)::bigint,
			COUNT(l.id)::bigint
		FROM wallets w
		LEFT JOIN wallet_ledger_entries l ON l.wallet_id = w.id
		WHERE w.id = $1
		GROUP BY w.id
	`, walletID).Scan(
		&id, &playerID, &currency, &storedInCents, &version,
		&createdAt, &updatedAt, &calculatedInCents, &checkedEntries,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ReconciliationResult{}, application.ErrWalletNotFound
	}
	if err != nil {
		return application.ReconciliationResult{}, fmt.Errorf("reconcile wallet: %w", err)
	}

	stored, err := moneyFromCents(storedInCents, currency)
	if err != nil {
		return application.ReconciliationResult{}, fmt.Errorf("restore stored balance: %w", err)
	}
	calculated, err := moneyFromCents(calculatedInCents, currency)
	if err != nil {
		return application.ReconciliationResult{}, fmt.Errorf("restore calculated balance: %w", err)
	}
	if _, err := domain.RehydrateWallet(id, playerID, stored, version, createdAt, updatedAt); err != nil {
		return application.ReconciliationResult{}, fmt.Errorf("rehydrate reconciled wallet: %w", err)
	}
	difference, err := stored.Subtract(calculated)
	if err != nil {
		return application.ReconciliationResult{}, fmt.Errorf("calculate reconciliation difference: %w", err)
	}
	return application.ReconciliationResult{
		WalletID: id, StoredBalance: stored, CalculatedBalance: calculated,
		Difference: difference, Consistent: difference.AmountInCents() == 0,
		CheckedEntries: checkedEntries,
	}, nil
}

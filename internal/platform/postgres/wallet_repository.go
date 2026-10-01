package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var ErrDatabaseExecutorRequired = errors.New("database executor is required")

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type WalletRepository struct {
	db DBTX
}

func NewWalletRepository(db DBTX) (*WalletRepository, error) {
	if db == nil {
		return nil, ErrDatabaseExecutorRequired
	}

	return &WalletRepository{db: db}, nil
}

func (repository *WalletRepository) InsertWallet(
	ctx context.Context,
	wallet domain.Wallet,
) error {
	_, err := repository.db.Exec(ctx, `
		INSERT INTO wallets (
			id,
			player_id,
			currency,
			balance_in_cents,
			version,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		wallet.ID(),
		wallet.PlayerID(),
		wallet.Currency(),
		wallet.Balance().AmountInCents(),
		wallet.Version(),
		wallet.CreatedAt(),
		wallet.UpdatedAt(),
	)
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: %v", application.ErrWalletAlreadyExists, err)
	}
	if err != nil {
		return fmt.Errorf("insert wallet: %w", err)
	}

	return nil
}

func (repository *WalletRepository) FindWalletForUpdate(
	ctx context.Context,
	walletID string,
) (domain.Wallet, error) {
	row := repository.db.QueryRow(ctx, `
		SELECT
			id,
			player_id,
			currency,
			balance_in_cents,
			version,
			created_at,
			updated_at
		FROM wallets
		WHERE id = $1
		FOR UPDATE
	`, walletID)

	var (
		id             string
		playerID       string
		currency       string
		balanceInCents int64
		version        int64
		createdAt      time.Time
		updatedAt      time.Time
	)

	err := row.Scan(
		&id,
		&playerID,
		&currency,
		&balanceInCents,
		&version,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Wallet{}, application.ErrWalletNotFound
	}
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("find wallet for update: %w", err)
	}

	balance, err := moneyFromCents(balanceInCents, currency)
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("restore wallet balance: %w", err)
	}

	wallet, err := domain.RehydrateWallet(
		id,
		playerID,
		balance,
		version,
		createdAt,
		updatedAt,
	)
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("rehydrate wallet: %w", err)
	}

	return wallet, nil
}

func (repository *WalletRepository) FindWallet(
	ctx context.Context,
	walletID string,
) (domain.Wallet, error) {
	return scanWallet(repository.db.QueryRow(ctx, `
		SELECT id, player_id, currency, balance_in_cents, version, created_at, updated_at
		FROM wallets
		WHERE id = $1
	`, walletID))
}

func scanWallet(row pgx.Row) (domain.Wallet, error) {
	var id, playerID, currency string
	var balanceInCents, version int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &playerID, &currency, &balanceInCents, &version, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Wallet{}, application.ErrWalletNotFound
		}
		return domain.Wallet{}, fmt.Errorf("scan wallet: %w", err)
	}
	balance, err := moneyFromCents(balanceInCents, currency)
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("restore wallet balance: %w", err)
	}
	wallet, err := domain.RehydrateWallet(id, playerID, balance, version, createdAt, updatedAt)
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("rehydrate wallet: %w", err)
	}
	return wallet, nil
}

func (repository *WalletRepository) WalletExistsForPlayerAndCurrency(
	ctx context.Context,
	playerID string,
	currency string,
) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM wallets
			WHERE player_id = $1 AND currency = $2
		)
	`, playerID, currency).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check wallet existence: %w", err)
	}

	return exists, nil
}

func (repository *WalletRepository) SaveWallet(
	ctx context.Context,
	wallet domain.Wallet,
) error {
	previousVersion := wallet.Version() - 1
	commandTag, err := repository.db.Exec(ctx, `
		UPDATE wallets
		SET
			balance_in_cents = $1,
			version = $2,
			updated_at = $3
		WHERE id = $4 AND version = $5
	`,
		wallet.Balance().AmountInCents(),
		wallet.Version(),
		wallet.UpdatedAt(),
		wallet.ID(),
		previousVersion,
	)
	if err != nil {
		return fmt.Errorf("save wallet: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return application.ErrConcurrentWalletUpdate
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func moneyFromCents(amountInCents int64, currency string) (domain.Money, error) {
	if amountInCents < 0 {
		return domain.Money{}, domain.ErrInvalidAmount
	}

	amount := fmt.Sprintf("%d.%02d", amountInCents/100, amountInCents%100)
	return domain.NewMoney(amount, currency)
}

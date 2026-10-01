package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

const selectWagerTransaction = `
	SELECT
		id,
		external_transaction_id,
		provider_id,
		idempotency_key,
		payload_hash,
		wallet_id,
		player_id,
		round_id,
		game_id,
		kind,
		amount_in_cents,
		currency,
		status,
		failure_code,
		reference_external_transaction_id,
		reference_transaction_id,
		result_balance_in_cents,
		result_balance_currency,
		reference_attempts,
		next_reference_attempt_at,
		created_at,
		updated_at
	FROM wager_transactions
`

type WagerTransactionRepository struct {
	db DBTX
}

func NewWagerTransactionRepository(db DBTX) (*WagerTransactionRepository, error) {
	if db == nil {
		return nil, ErrDatabaseExecutorRequired
	}

	return &WagerTransactionRepository{db: db}, nil
}

func (repository *WagerTransactionRepository) FindWagerTransactionForUpdate(
	ctx context.Context,
	transactionID string,
) (domain.WagerTransaction, error) {
	return scanWagerTransaction(repository.db.QueryRow(
		ctx,
		selectWagerTransaction+" WHERE id = $1 FOR UPDATE",
		transactionID,
	))
}

func (repository *WagerTransactionRepository) FindWagerTransactionForProvider(
	ctx context.Context,
	providerID string,
	transactionID string,
) (domain.WagerTransaction, error) {
	return scanWagerTransaction(repository.db.QueryRow(
		ctx,
		selectWagerTransaction+" WHERE provider_id = $1 AND id = $2",
		providerID,
		transactionID,
	))
}

func (repository *WagerTransactionRepository) FindWagerTransactionByExternalID(
	ctx context.Context,
	providerID string,
	externalTransactionID string,
) (domain.WagerTransaction, error) {
	return scanWagerTransaction(repository.db.QueryRow(
		ctx,
		selectWagerTransaction+" WHERE provider_id = $1 AND external_transaction_id = $2",
		providerID,
		externalTransactionID,
	))
}

func (repository *WagerTransactionRepository) FindWagerTransactionByIdempotencyKeyForUpdate(
	ctx context.Context,
	providerID string,
	idempotencyKey string,
) (domain.WagerTransaction, error) {
	return scanWagerTransaction(repository.db.QueryRow(
		ctx,
		selectWagerTransaction+
			" WHERE provider_id = $1 AND idempotency_key = $2 FOR UPDATE",
		providerID,
		idempotencyKey,
	))
}

func (repository *WagerTransactionRepository) FindWagerTransactionByExternalIDForUpdate(
	ctx context.Context,
	providerID string,
	externalTransactionID string,
) (domain.WagerTransaction, error) {
	return scanWagerTransaction(repository.db.QueryRow(
		ctx,
		selectWagerTransaction+
			" WHERE provider_id = $1 AND external_transaction_id = $2 FOR UPDATE",
		providerID,
		externalTransactionID,
	))
}

func (repository *WagerTransactionRepository) HasProcessedReversal(
	ctx context.Context,
	referenceTransactionID string,
	kind domain.WagerTransactionKind,
) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM wager_transactions
			WHERE reference_transaction_id = $1
				AND kind = $2
				AND status = 'PROCESSED'
		)
	`, referenceTransactionID, string(kind)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check processed reversal: %w", err)
	}

	return exists, nil
}

func (repository *WagerTransactionRepository) InsertWagerTransaction(
	ctx context.Context,
	transaction domain.WagerTransaction,
) error {
	resultBalanceInCents, resultBalanceCurrency := nullableResultBalance(transaction)
	nextReferenceAttemptAt := nullableNextReferenceAttemptAt(transaction)

	_, err := repository.db.Exec(ctx, `
		INSERT INTO wager_transactions (
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_in_cents,
			currency,
			status,
			failure_code,
			reference_external_transaction_id,
			reference_transaction_id,
			result_balance_in_cents,
			result_balance_currency,
			reference_attempts,
			next_reference_attempt_at,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
	`,
		transaction.ID(),
		nullableString(transaction.ExternalTransactionID()),
		nullableString(transaction.ProviderID()),
		nullableString(transaction.IdempotencyKey()),
		nullableString(transaction.PayloadHash()),
		transaction.WalletID(),
		transaction.PlayerID(),
		nullableString(transaction.RoundID()),
		nullableString(transaction.GameID()),
		string(transaction.Kind()),
		transaction.Money().AmountInCents(),
		transaction.Money().Currency(),
		string(transaction.Status()),
		nullableString(string(transaction.FailureCode())),
		nullableString(transaction.ReferenceExternalTransactionID()),
		nullableString(transaction.ReferenceTransactionID()),
		resultBalanceInCents,
		resultBalanceCurrency,
		transaction.ReferenceAttempts(),
		nextReferenceAttemptAt,
		transaction.CreatedAt(),
		transaction.UpdatedAt(),
	)
	if err != nil {
		return mapWagerTransactionInsertError(err)
	}

	return nil
}

func (repository *WagerTransactionRepository) SaveWagerTransaction(
	ctx context.Context,
	transaction domain.WagerTransaction,
) error {
	resultBalanceInCents, resultBalanceCurrency := nullableResultBalance(transaction)
	nextReferenceAttemptAt := nullableNextReferenceAttemptAt(transaction)

	commandTag, err := repository.db.Exec(ctx, `
		UPDATE wager_transactions
		SET
			status = $1,
			failure_code = $2,
			reference_transaction_id = $3,
			result_balance_in_cents = $4,
			result_balance_currency = $5,
			reference_attempts = $6,
			next_reference_attempt_at = $7,
			updated_at = $8
		WHERE id = $9
	`,
		string(transaction.Status()),
		nullableString(string(transaction.FailureCode())),
		nullableString(transaction.ReferenceTransactionID()),
		resultBalanceInCents,
		resultBalanceCurrency,
		transaction.ReferenceAttempts(),
		nextReferenceAttemptAt,
		transaction.UpdatedAt(),
		transaction.ID(),
	)
	if err != nil {
		return fmt.Errorf("save wager transaction: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return application.ErrWagerTransactionNotFound
	}

	return nil
}

func scanWagerTransaction(row pgx.Row) (domain.WagerTransaction, error) {
	var (
		id                             string
		externalTransactionID          pgtype.Text
		providerID                     pgtype.Text
		idempotencyKey                 pgtype.Text
		payloadHash                    pgtype.Text
		walletID                       string
		playerID                       string
		roundID                        pgtype.Text
		gameID                         pgtype.Text
		kind                           string
		amountInCents                  int64
		currency                       string
		status                         string
		failureCode                    pgtype.Text
		referenceExternalTransactionID pgtype.Text
		referenceTransactionID         pgtype.Text
		resultBalanceInCents           pgtype.Int8
		resultBalanceCurrency          pgtype.Text
		referenceAttempts              int
		nextReferenceAttemptAt         pgtype.Timestamptz
		createdAt                      time.Time
		updatedAt                      time.Time
	)

	err := row.Scan(
		&id,
		&externalTransactionID,
		&providerID,
		&idempotencyKey,
		&payloadHash,
		&walletID,
		&playerID,
		&roundID,
		&gameID,
		&kind,
		&amountInCents,
		&currency,
		&status,
		&failureCode,
		&referenceExternalTransactionID,
		&referenceTransactionID,
		&resultBalanceInCents,
		&resultBalanceCurrency,
		&referenceAttempts,
		&nextReferenceAttemptAt,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WagerTransaction{}, application.ErrWagerTransactionNotFound
	}
	if err != nil {
		return domain.WagerTransaction{}, fmt.Errorf("scan wager transaction: %w", err)
	}

	money, err := moneyFromCents(amountInCents, currency)
	if err != nil {
		return domain.WagerTransaction{}, fmt.Errorf("restore wager transaction money: %w", err)
	}

	var resultBalance domain.Money
	if resultBalanceInCents.Valid {
		resultBalance, err = moneyFromCents(
			resultBalanceInCents.Int64,
			resultBalanceCurrency.String,
		)
		if err != nil {
			return domain.WagerTransaction{}, fmt.Errorf(
				"restore wager transaction result balance: %w",
				err,
			)
		}
	}

	transaction, err := domain.RehydrateWagerTransaction(domain.RehydrateWagerTransactionParams{
		ID:                             id,
		ExternalTransactionID:          externalTransactionID.String,
		ProviderID:                     providerID.String,
		IdempotencyKey:                 idempotencyKey.String,
		PayloadHash:                    payloadHash.String,
		WalletID:                       walletID,
		PlayerID:                       playerID,
		RoundID:                        roundID.String,
		GameID:                         gameID.String,
		Kind:                           domain.WagerTransactionKind(kind),
		Money:                          money,
		Status:                         domain.WagerTransactionStatus(status),
		ReferenceExternalTransactionID: referenceExternalTransactionID.String,
		ReferenceTransactionID:         referenceTransactionID.String,
		ResultBalance:                  resultBalance,
		HasResultBalance:               resultBalanceInCents.Valid,
		FailureCode:                    domain.WagerTransactionFailureCode(failureCode.String),
		ReferenceAttempts:              referenceAttempts,
		NextReferenceAttemptAt:         nextReferenceAttemptAt.Time,
		HasNextReferenceAttempt:        nextReferenceAttemptAt.Valid,
		CreatedAt:                      createdAt,
		UpdatedAt:                      updatedAt,
	})
	if err != nil {
		return domain.WagerTransaction{}, fmt.Errorf("rehydrate wager transaction: %w", err)
	}

	return transaction, nil
}

func nullableResultBalance(transaction domain.WagerTransaction) (any, any) {
	resultBalance, ok := transaction.ResultBalance()
	if !ok {
		return nil, nil
	}

	return resultBalance.AmountInCents(), resultBalance.Currency()
}

func nullableNextReferenceAttemptAt(transaction domain.WagerTransaction) any {
	nextAttemptAt, ok := transaction.NextReferenceAttemptAt()
	if !ok {
		return nil
	}

	return nextAttemptAt
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func mapWagerTransactionInsertError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23505" {
		return fmt.Errorf("insert wager transaction: %w", err)
	}

	switch postgresError.ConstraintName {
	case "wager_transactions_provider_idempotency_uidx":
		return fmt.Errorf("%w: %v", application.ErrIdempotencyKeyConflict, err)
	case "wager_transactions_provider_external_uidx":
		return fmt.Errorf("%w: %v", application.ErrExternalTransactionConflict, err)
	default:
		return fmt.Errorf("%w: %v", application.ErrWagerTransactionAlreadyExists, err)
	}
}

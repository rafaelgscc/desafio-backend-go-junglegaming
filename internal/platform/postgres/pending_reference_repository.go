package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

const maxReferenceErrorLength = 2000

type PendingReferenceRepository struct {
	pool *pgxpool.Pool
}

func NewPendingReferenceRepository(pool *pgxpool.Pool) (*PendingReferenceRepository, error) {
	if pool == nil {
		return nil, ErrPostgresPoolRequired
	}
	return &PendingReferenceRepository{pool: pool}, nil
}

func (repository *PendingReferenceRepository) ClaimPendingReferences(
	ctx context.Context,
	owner string,
	now time.Time,
	leaseExpiresAt time.Time,
	limit int,
) ([]application.PendingReference, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin pending reference claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM wager_transactions
			WHERE status = 'PENDING_REFERENCE'
				AND kind IN ('REFUND', 'ROLLBACK')
				AND next_reference_attempt_at <= $1
				AND (
					reference_lease_expires_at IS NULL
					OR reference_lease_expires_at <= $1
				)
			ORDER BY next_reference_attempt_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		UPDATE wager_transactions AS transaction
		SET reference_lease_owner = $3,
			reference_lease_expires_at = $4,
			reference_last_error = NULL
		FROM candidates
		WHERE transaction.id = candidates.id
		RETURNING transaction.id, transaction.kind, transaction.reference_attempts
	`, now.UTC(), limit, owner, leaseExpiresAt.UTC())
	if err != nil {
		return nil, fmt.Errorf("claim pending references: %w", err)
	}
	defer rows.Close()

	claimed := make([]application.PendingReference, 0, limit)
	for rows.Next() {
		var reference application.PendingReference
		if err := rows.Scan(
			&reference.TransactionID,
			&reference.Kind,
			&reference.ReferenceAttempts,
		); err != nil {
			return nil, fmt.Errorf("scan claimed pending reference: %w", err)
		}
		claimed = append(claimed, reference)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed pending references: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit pending reference claim: %w", err)
	}
	return claimed, nil
}

func (repository *PendingReferenceRepository) ReleasePendingReference(
	ctx context.Context,
	transactionID string,
	owner string,
) error {
	tag, err := repository.pool.Exec(ctx, `
		UPDATE wager_transactions
		SET reference_lease_owner = NULL,
			reference_lease_expires_at = NULL,
			reference_last_error = NULL
		WHERE id = $1 AND reference_lease_owner = $2
	`, transactionID, owner)
	if err != nil {
		return fmt.Errorf("release pending reference: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return application.ErrReferenceLeaseLost
	}
	return nil
}

func (repository *PendingReferenceRepository) FailPendingReference(
	ctx context.Context,
	transactionID string,
	owner string,
	nextAttemptAt time.Time,
	errorMessage string,
) error {
	if len(errorMessage) > maxReferenceErrorLength {
		errorMessage = errorMessage[:maxReferenceErrorLength]
	}
	tag, err := repository.pool.Exec(ctx, `
		UPDATE wager_transactions
		SET next_reference_attempt_at = $1,
			reference_last_error = $2,
			reference_lease_owner = NULL,
			reference_lease_expires_at = NULL
		WHERE id = $3
			AND reference_lease_owner = $4
			AND status = 'PENDING_REFERENCE'
	`, nextAttemptAt.UTC(), errorMessage, transactionID, owner)
	if err != nil {
		return fmt.Errorf("fail pending reference: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return application.ErrReferenceLeaseLost
	}
	return nil
}

var _ application.PendingReferenceRepository = (*PendingReferenceRepository)(nil)

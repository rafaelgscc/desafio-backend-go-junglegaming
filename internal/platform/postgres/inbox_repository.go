package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

type InboxRepository struct {
	pool *pgxpool.Pool
}

func NewInboxRepository(pool *pgxpool.Pool) (*InboxRepository, error) {
	if pool == nil {
		return nil, ErrPostgresPoolRequired
	}
	return &InboxRepository{pool: pool}, nil
}

func (repository *InboxRepository) Claim(
	ctx context.Context,
	message application.InboxMessage,
	leaseOwner string,
	leaseExpiresAt time.Time,
) (application.InboxClaimStatus, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("begin inbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	commandTag, err := tx.Exec(ctx, `
		INSERT INTO inbox_events (
			consumer_name, message_id, event_id, event_type, payload_hash, payload,
			received_at, processing_attempts, next_processing_attempt_at,
			lease_owner, lease_expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $7, $8, $9)
		ON CONFLICT (consumer_name, message_id) DO NOTHING
	`, message.ConsumerName, message.MessageID, message.EventID, message.EventType,
		message.PayloadHash, message.Payload, message.ReceivedAt.UTC(), leaseOwner,
		leaseExpiresAt.UTC(),
	)
	if err != nil {
		return "", fmt.Errorf("insert inbox message: %w", err)
	}
	if commandTag.RowsAffected() == 1 {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit inbox claim: %w", err)
		}
		return application.InboxClaimed, nil
	}

	var payloadHash string
	var processedAt, currentLeaseExpiresAt *time.Time
	var currentLeaseOwner *string
	err = tx.QueryRow(ctx, `
		SELECT payload_hash, processed_at, lease_owner, lease_expires_at
		FROM inbox_events
		WHERE consumer_name = $1 AND message_id = $2
		FOR UPDATE
	`, message.ConsumerName, message.MessageID).Scan(
		&payloadHash, &processedAt, &currentLeaseOwner, &currentLeaseExpiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("lock inbox message: %w", err)
	}
	if payloadHash != message.PayloadHash {
		return "", application.ErrInboxPayloadConflict
	}
	if processedAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit completed inbox lookup: %w", err)
		}
		return application.InboxAlreadyProcessed, nil
	}
	now := message.ReceivedAt.UTC()
	if currentLeaseOwner != nil && currentLeaseExpiresAt != nil &&
		*currentLeaseOwner != leaseOwner && currentLeaseExpiresAt.After(now) {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit busy inbox lookup: %w", err)
		}
		return application.InboxBusy, nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE inbox_events
		SET processing_attempts = processing_attempts + 1,
			next_processing_attempt_at = $1,
			lease_owner = $2,
			lease_expires_at = $3,
			last_error = NULL
		WHERE consumer_name = $4 AND message_id = $5
	`, now, leaseOwner, leaseExpiresAt.UTC(), message.ConsumerName, message.MessageID)
	if err != nil {
		return "", fmt.Errorf("renew inbox claim: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit inbox reclaim: %w", err)
	}
	return application.InboxClaimed, nil
}

func (repository *InboxRepository) Complete(
	ctx context.Context,
	consumerName string,
	messageID string,
	leaseOwner string,
	processedAt time.Time,
) error {
	tag, err := repository.pool.Exec(ctx, `
		UPDATE inbox_events
		SET processed_at = $1, lease_owner = NULL, lease_expires_at = NULL, last_error = NULL
		WHERE consumer_name = $2 AND message_id = $3
			AND lease_owner = $4 AND processed_at IS NULL
	`, processedAt.UTC(), consumerName, messageID, leaseOwner)
	if err != nil {
		return fmt.Errorf("complete inbox message: %w", err)
	}
	if tag.RowsAffected() != 1 {
		var completed bool
		if err := repository.pool.QueryRow(ctx, `
			SELECT processed_at IS NOT NULL
			FROM inbox_events
			WHERE consumer_name = $1 AND message_id = $2
		`, consumerName, messageID).Scan(&completed); err == nil && completed {
			return nil
		}
		return application.ErrInboxLeaseLost
	}
	return nil
}

func (repository *InboxRepository) Fail(
	ctx context.Context,
	consumerName string,
	messageID string,
	leaseOwner string,
	nextAttemptAt time.Time,
	lastError string,
) error {
	if len(lastError) > 2000 {
		lastError = lastError[:2000]
	}
	tag, err := repository.pool.Exec(ctx, `
		UPDATE inbox_events
		SET next_processing_attempt_at = $1, lease_owner = NULL, lease_expires_at = NULL,
			last_error = $2
		WHERE consumer_name = $3 AND message_id = $4
			AND lease_owner = $5 AND processed_at IS NULL
	`, nextAttemptAt.UTC(), lastError, consumerName, messageID, leaseOwner)
	if err != nil {
		return fmt.Errorf("fail inbox message: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return application.ErrInboxLeaseLost
	}
	return nil
}

var _ application.InboxRepository = (*InboxRepository)(nil)

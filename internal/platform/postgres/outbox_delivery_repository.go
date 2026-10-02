package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

type OutboxDeliveryRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxDeliveryRepository(pool *pgxpool.Pool) (*OutboxDeliveryRepository, error) {
	if pool == nil {
		return nil, ErrPostgresPoolRequired
	}
	return &OutboxDeliveryRepository{pool: pool}, nil
}

func (repository *OutboxDeliveryRepository) ClaimOutboxEvents(
	ctx context.Context,
	leaseOwner string,
	now time.Time,
	leaseExpiresAt time.Time,
	limit int,
) ([]application.OutboxEvent, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT current.event_id
			FROM outbox_events current
			WHERE current.published_at IS NULL
				AND current.next_publish_attempt_at <= $1
				AND (current.lease_expires_at IS NULL OR current.lease_expires_at <= $1)
				AND NOT EXISTS (
					SELECT 1
					FROM outbox_events previous
					WHERE previous.aggregate_id = current.aggregate_id
						AND previous.published_at IS NULL
						AND (previous.occurred_at, previous.event_id) <
							(current.occurred_at, current.event_id)
				)
			ORDER BY current.next_publish_attempt_at, current.occurred_at, current.event_id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		UPDATE outbox_events event
		SET lease_owner = $3,
			lease_expires_at = $4,
			publish_attempts = event.publish_attempts + 1
		FROM candidates
		WHERE event.event_id = candidates.event_id
		RETURNING event.event_id, event.event_type, event.aggregate_id,
			event.correlation_id, event.causation_id, event.occurred_at,
			event.version, event.payload, event.publish_attempts
	`, now.UTC(), limit, leaseOwner, leaseExpiresAt.UTC())
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer rows.Close()
	events := make([]application.OutboxEvent, 0)
	for rows.Next() {
		var event application.OutboxEvent
		var eventType string
		var causationID pgtype.Text
		if err := rows.Scan(
			&event.EventID, &eventType, &event.AggregateID, &event.CorrelationID,
			&causationID, &event.OccurredAt, &event.Version, &event.Data,
			&event.PublishAttempts,
		); err != nil {
			return nil, fmt.Errorf("scan claimed outbox event: %w", err)
		}
		event.EventType = application.IntegrationEventType(eventType)
		if causationID.Valid {
			event.CausationID = causationID.String
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed outbox events: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return events, nil
}

func (repository *OutboxDeliveryRepository) MarkOutboxEventPublished(
	ctx context.Context,
	eventID string,
	leaseOwner string,
	publishedAt time.Time,
) error {
	tag, err := repository.pool.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = $1, lease_owner = NULL, lease_expires_at = NULL,
			last_error = NULL
		WHERE event_id = $2 AND lease_owner = $3 AND published_at IS NULL
	`, publishedAt.UTC(), eventID, leaseOwner)
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return application.ErrOutboxLeaseLost
	}
	return nil
}

func (repository *OutboxDeliveryRepository) MarkOutboxEventFailed(
	ctx context.Context,
	eventID string,
	leaseOwner string,
	nextAttemptAt time.Time,
	lastError string,
) error {
	if len(lastError) > 2000 {
		lastError = lastError[:2000]
	}
	tag, err := repository.pool.Exec(ctx, `
		UPDATE outbox_events
		SET next_publish_attempt_at = $1, last_error = $2,
			lease_owner = NULL, lease_expires_at = NULL
		WHERE event_id = $3 AND lease_owner = $4 AND published_at IS NULL
	`, nextAttemptAt.UTC(), lastError, eventID, leaseOwner)
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return application.ErrOutboxLeaseLost
	}
	return nil
}

var _ application.OutboxDeliveryRepository = (*OutboxDeliveryRepository)(nil)

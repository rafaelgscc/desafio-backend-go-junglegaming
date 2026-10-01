package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

type OutboxRepository struct {
	db DBTX
}

func NewOutboxRepository(db DBTX) (*OutboxRepository, error) {
	if db == nil {
		return nil, ErrDatabaseExecutorRequired
	}

	return &OutboxRepository{db: db}, nil
}

func (repository *OutboxRepository) AppendOutboxEvent(
	ctx context.Context,
	event application.IntegrationEvent,
) error {
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("encode outbox event payload: %w", err)
	}

	_, err = repository.db.Exec(ctx, `
		INSERT INTO outbox_events (
			event_id,
			event_type,
			aggregate_id,
			correlation_id,
			causation_id,
			occurred_at,
			version,
			payload
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		event.EventID,
		string(event.EventType),
		event.AggregateID,
		event.CorrelationID,
		nullableString(event.CausationID),
		event.OccurredAt,
		event.Version,
		string(payload),
	)
	if isOutboxEventUniqueViolation(err) {
		return fmt.Errorf("%w: %v", application.ErrOutboxEventAlreadyExists, err)
	}
	if err != nil {
		return fmt.Errorf("append outbox event: %w", err)
	}

	return nil
}

func isOutboxEventUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

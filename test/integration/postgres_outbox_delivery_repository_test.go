package integration_test

import (
	"context"
	"errors"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestOutboxDeliveryRepositoryCoordinatesPublishersAndPreservesAggregateOrder(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL: databaseURL, MaxConnections: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schemaName := "outbox_delivery_repository_test"
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE") }()
	if _, err := pool.Exec(ctx, "SET search_path TO "+quotedSchemaName); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{
		"000004_create_outbox_events.up.sql", "000006_add_outbox_delivery_control.up.sql",
	} {
		if _, err := pool.Exec(ctx, readMigrationFile(t, migration)); err != nil {
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}
	now := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)
	for _, fixture := range []struct {
		id, aggregate string
		occurredAt    time.Time
	}{
		{id: "event-a-1", aggregate: "wallet-a", occurredAt: now},
		{id: "event-a-2", aggregate: "wallet-a", occurredAt: now.Add(time.Second)},
		{id: "event-b-1", aggregate: "wallet-b", occurredAt: now},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO outbox_events (
				event_id, event_type, aggregate_id, correlation_id,
				occurred_at, version, payload, next_publish_attempt_at
			) VALUES ($1, 'WalletBalanceChanged', $2, 'correlation-1', $3, 1, '{}'::jsonb, $4)
		`, fixture.id, fixture.aggregate, fixture.occurredAt, now); err != nil {
			t.Fatalf("insert fixture %s: %v", fixture.id, err)
		}
	}
	repository, err := platformpostgres.NewOutboxDeliveryRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.ClaimOutboxEvents(
		ctx, "publisher-1", now, now.Add(30*time.Second), 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	ids := outboxEventIDs(first)
	if len(ids) != 2 || ids[0] != "event-a-1" || ids[1] != "event-b-1" {
		t.Fatalf("first claims = %v", ids)
	}
	secondPublisher, err := repository.ClaimOutboxEvents(
		ctx, "publisher-2", now, now.Add(30*time.Second), 10,
	)
	if err != nil || len(secondPublisher) != 0 {
		t.Fatalf("competing claims = %#v, error %v", secondPublisher, err)
	}
	if err := repository.MarkOutboxEventPublished(ctx, "event-a-1", "publisher-1", now); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkOutboxEventFailed(
		ctx, "event-b-1", "publisher-1", now.Add(10*time.Second), "SQS unavailable",
	); err != nil {
		t.Fatal(err)
	}
	next, err := repository.ClaimOutboxEvents(
		ctx, "publisher-1", now.Add(time.Second), now.Add(31*time.Second), 10,
	)
	if err != nil || len(next) != 1 || next[0].EventID != "event-a-2" {
		t.Fatalf("next aggregate claim = %#v, error %v", next, err)
	}
	if err := repository.MarkOutboxEventPublished(
		ctx, "event-a-2", "publisher-2", now.Add(time.Second),
	); !errors.Is(err, application.ErrOutboxLeaseLost) {
		t.Fatalf("wrong-owner confirmation error = %v", err)
	}
	reclaimed, err := repository.ClaimOutboxEvents(
		ctx, "publisher-2", now.Add(32*time.Second), now.Add(time.Minute), 10,
	)
	if err != nil || len(reclaimed) != 2 {
		t.Fatalf("expired/retry claims = %#v, error %v", reclaimed, err)
	}
	for _, event := range reclaimed {
		if event.PublishAttempts != 2 {
			t.Fatalf("event %s attempts = %d, want 2", event.EventID, event.PublishAttempts)
		}
		if err := repository.MarkOutboxEventPublished(
			ctx, event.EventID, "publisher-2", now.Add(32*time.Second),
		); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO outbox_events (
			event_id, event_type, aggregate_id, correlation_id,
			occurred_at, version, payload, next_publish_attempt_at
		) VALUES (
			'event-concurrent', 'WalletBalanceChanged', 'wallet-concurrent',
			'correlation-1', $1, 1, '{}'::jsonb, $1
		)
	`, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	secondPool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL: databaseURL, MaxConnections: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer secondPool.Close()
	if _, err := secondPool.Exec(ctx, "SET search_path TO "+quotedSchemaName); err != nil {
		t.Fatal(err)
	}
	secondRepository, _ := platformpostgres.NewOutboxDeliveryRepository(secondPool)
	type claimResult struct {
		events []application.OutboxEvent
		err    error
	}
	start := make(chan struct{})
	results := make(chan claimResult, 2)
	claim := func(repository *platformpostgres.OutboxDeliveryRepository, owner string) {
		<-start
		events, err := repository.ClaimOutboxEvents(
			ctx, owner, now.Add(time.Minute), now.Add(90*time.Second), 1,
		)
		results <- claimResult{events: events, err: err}
	}
	go claim(repository, "publisher-1")
	go claim(secondRepository, "publisher-2")
	close(start)
	totalClaims := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		totalClaims += len(result.events)
		if len(result.events) == 1 && result.events[0].EventID != "event-concurrent" {
			t.Fatalf("unexpected concurrent claim: %#v", result.events)
		}
	}
	if totalClaims != 1 {
		t.Fatalf("concurrent publishers claimed event %d times, want 1", totalClaims)
	}
}

func outboxEventIDs(events []application.OutboxEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.EventID)
	}
	sort.Strings(ids)
	return ids
}

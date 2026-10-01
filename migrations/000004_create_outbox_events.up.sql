CREATE TABLE outbox_events (
    event_id TEXT PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    aggregate_id TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    causation_id TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    version INTEGER NOT NULL,
    payload JSONB NOT NULL,
    published_at TIMESTAMPTZ,
    publish_attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT outbox_events_event_id_not_blank
        CHECK (BTRIM(event_id) <> ''),
    CONSTRAINT outbox_events_type_valid
        CHECK (
            event_type IN (
                'WagerTransactionProcessed',
                'WagerTransactionRejected',
                'WalletBalanceChanged',
                'WagerTransactionPendingReference'
            )
        ),
    CONSTRAINT outbox_events_aggregate_id_not_blank
        CHECK (BTRIM(aggregate_id) <> ''),
    CONSTRAINT outbox_events_correlation_id_not_blank
        CHECK (BTRIM(correlation_id) <> ''),
    CONSTRAINT outbox_events_causation_id_not_blank
        CHECK (causation_id IS NULL OR BTRIM(causation_id) <> ''),
    CONSTRAINT outbox_events_version_positive
        CHECK (version > 0),
    CONSTRAINT outbox_events_payload_is_object
        CHECK (JSONB_TYPEOF(payload) = 'object'),
    CONSTRAINT outbox_events_publish_attempts_not_negative
        CHECK (publish_attempts >= 0)
);

CREATE INDEX outbox_events_unpublished_idx
    ON outbox_events (occurred_at, event_id)
    WHERE published_at IS NULL;

CREATE INDEX outbox_events_aggregate_idx
    ON outbox_events (aggregate_id, occurred_at, event_id);

CREATE INDEX outbox_events_correlation_idx
    ON outbox_events (correlation_id, occurred_at, event_id);

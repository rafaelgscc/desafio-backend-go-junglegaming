CREATE TABLE inbox_events (
    message_id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    processing_attempts INTEGER NOT NULL DEFAULT 0,
    next_processing_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner TEXT,
    lease_expires_at TIMESTAMPTZ,
    last_error TEXT,

    CONSTRAINT inbox_events_message_id_not_blank
        CHECK (BTRIM(message_id) <> ''),
    CONSTRAINT inbox_events_event_id_not_blank
        CHECK (BTRIM(event_id) <> ''),
    CONSTRAINT inbox_events_event_type_not_blank
        CHECK (BTRIM(event_type) <> ''),
    CONSTRAINT inbox_events_payload_is_object
        CHECK (JSONB_TYPEOF(payload) = 'object'),
    CONSTRAINT inbox_events_processing_attempts_not_negative
        CHECK (processing_attempts >= 0),
    CONSTRAINT inbox_events_lease_complete
        CHECK (
            (
                lease_owner IS NULL
                AND lease_expires_at IS NULL
            )
            OR (
                lease_owner IS NOT NULL
                AND BTRIM(lease_owner) <> ''
                AND lease_expires_at IS NOT NULL
            )
        ),
    CONSTRAINT inbox_events_processed_not_leased
        CHECK (
            processed_at IS NULL
            OR (
                lease_owner IS NULL
                AND lease_expires_at IS NULL
            )
        )
);

CREATE INDEX inbox_events_processable_idx
    ON inbox_events (
        next_processing_attempt_at,
        lease_expires_at,
        received_at,
        message_id
    )
    WHERE processed_at IS NULL;

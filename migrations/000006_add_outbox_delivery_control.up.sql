ALTER TABLE outbox_events
    ADD COLUMN next_publish_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN lease_owner TEXT,
    ADD COLUMN lease_expires_at TIMESTAMPTZ,
    ADD CONSTRAINT outbox_events_lease_complete
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
    ADD CONSTRAINT outbox_events_published_not_leased
        CHECK (
            published_at IS NULL
            OR (
                lease_owner IS NULL
                AND lease_expires_at IS NULL
            )
        );

DROP INDEX outbox_events_unpublished_idx;

CREATE INDEX outbox_events_publishable_idx
    ON outbox_events (
        next_publish_attempt_at,
        lease_expires_at,
        occurred_at,
        event_id
    )
    WHERE published_at IS NULL;

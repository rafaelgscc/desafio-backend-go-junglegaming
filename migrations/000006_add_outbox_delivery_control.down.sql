DROP INDEX outbox_events_publishable_idx;

CREATE INDEX outbox_events_unpublished_idx
    ON outbox_events (occurred_at, event_id)
    WHERE published_at IS NULL;

ALTER TABLE outbox_events
    DROP CONSTRAINT outbox_events_published_not_leased,
    DROP CONSTRAINT outbox_events_lease_complete,
    DROP COLUMN lease_expires_at,
    DROP COLUMN lease_owner,
    DROP COLUMN next_publish_attempt_at;

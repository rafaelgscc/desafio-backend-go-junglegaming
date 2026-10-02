ALTER TABLE inbox_events
    ADD COLUMN consumer_name TEXT,
    ADD COLUMN payload_hash TEXT;

UPDATE inbox_events
SET
    consumer_name = 'wager-transactions',
    payload_hash = 'legacy-md5:' || MD5(payload::text);

ALTER TABLE inbox_events
    ALTER COLUMN consumer_name SET NOT NULL,
    ALTER COLUMN payload_hash SET NOT NULL,
    DROP CONSTRAINT inbox_events_pkey,
    ADD CONSTRAINT inbox_events_pkey PRIMARY KEY (consumer_name, message_id),
    ADD CONSTRAINT inbox_events_consumer_name_not_blank
        CHECK (BTRIM(consumer_name) <> ''),
    ADD CONSTRAINT inbox_events_payload_hash_not_blank
        CHECK (BTRIM(payload_hash) <> '');


ALTER TABLE inbox_events
    DROP CONSTRAINT inbox_events_pkey,
    DROP CONSTRAINT inbox_events_consumer_name_not_blank,
    DROP CONSTRAINT inbox_events_payload_hash_not_blank,
    ADD CONSTRAINT inbox_events_pkey PRIMARY KEY (message_id),
    DROP COLUMN consumer_name,
    DROP COLUMN payload_hash;

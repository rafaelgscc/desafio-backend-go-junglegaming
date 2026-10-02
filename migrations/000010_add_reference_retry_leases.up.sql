ALTER TABLE wager_transactions
    ADD COLUMN reference_lease_owner TEXT,
    ADD COLUMN reference_lease_expires_at TIMESTAMPTZ,
    ADD COLUMN reference_last_error TEXT,
    ADD CONSTRAINT wager_transactions_reference_lease_complete
        CHECK (
            (
                reference_lease_owner IS NULL
                AND reference_lease_expires_at IS NULL
            )
            OR (
                reference_lease_owner IS NOT NULL
                AND BTRIM(reference_lease_owner) <> ''
                AND reference_lease_expires_at IS NOT NULL
            )
        );

CREATE INDEX wager_transactions_reference_retry_claim_idx
    ON wager_transactions (
        next_reference_attempt_at,
        reference_lease_expires_at,
        id
    )
    WHERE status = 'PENDING_REFERENCE';


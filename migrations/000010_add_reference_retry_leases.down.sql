DROP INDEX wager_transactions_reference_retry_claim_idx;

ALTER TABLE wager_transactions
    DROP CONSTRAINT wager_transactions_reference_lease_complete,
    DROP COLUMN reference_lease_owner,
    DROP COLUMN reference_lease_expires_at,
    DROP COLUMN reference_last_error;

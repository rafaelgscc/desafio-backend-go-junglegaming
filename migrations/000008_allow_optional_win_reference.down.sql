ALTER TABLE wager_transactions
    DROP CONSTRAINT wager_transactions_reference_metadata_valid;

ALTER TABLE wager_transactions
    ADD CONSTRAINT wager_transactions_reference_metadata_valid
        CHECK (
            (
                kind IN ('REFUND', 'ROLLBACK')
                AND reference_external_transaction_id IS NOT NULL
                AND BTRIM(reference_external_transaction_id) <> ''
            )
            OR (
                kind NOT IN ('REFUND', 'ROLLBACK')
                AND reference_external_transaction_id IS NULL
                AND reference_transaction_id IS NULL
            )
        ) NOT VALID;

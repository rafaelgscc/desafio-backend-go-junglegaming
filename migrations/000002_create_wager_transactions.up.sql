CREATE TABLE wager_transactions (
    id TEXT PRIMARY KEY,
    external_transaction_id TEXT,
    provider_id TEXT,
    idempotency_key TEXT,
    payload_hash TEXT,
    wallet_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    round_id TEXT,
    game_id TEXT,
    kind VARCHAR(16) NOT NULL,
    amount_in_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(24) NOT NULL,
    failure_code VARCHAR(40),
    reference_external_transaction_id TEXT,
    reference_transaction_id TEXT,
    result_balance_in_cents BIGINT,
    result_balance_currency VARCHAR(3),
    reference_attempts INTEGER NOT NULL DEFAULT 0,
    next_reference_attempt_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT wager_transactions_wallet_fk
        FOREIGN KEY (wallet_id) REFERENCES wallets (id),
    CONSTRAINT wager_transactions_reference_fk
        FOREIGN KEY (reference_transaction_id) REFERENCES wager_transactions (id),
    CONSTRAINT wager_transactions_id_not_blank
        CHECK (BTRIM(id) <> ''),
    CONSTRAINT wager_transactions_player_id_not_blank
        CHECK (BTRIM(player_id) <> ''),
    CONSTRAINT wager_transactions_currency_format
        CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT wager_transactions_kind_valid
        CHECK (kind IN ('OPENING', 'BET', 'WIN', 'LOSS', 'REFUND', 'ROLLBACK')),
    CONSTRAINT wager_transactions_amount_valid
        CHECK (
            (kind = 'LOSS' AND amount_in_cents = 0)
            OR (kind <> 'LOSS' AND amount_in_cents > 0)
        ),
    CONSTRAINT wager_transactions_status_valid
        CHECK (status IN ('PENDING', 'PENDING_REFERENCE', 'PROCESSED', 'REJECTED', 'FAILED')),
    CONSTRAINT wager_transactions_failure_code_valid
        CHECK (
            failure_code IS NULL
            OR failure_code IN (
                'REFERENCE_NOT_FOUND',
                'INFRASTRUCTURE_FAILURE',
                'INSUFFICIENT_FUNDS',
                'REVERSAL_INSUFFICIENT_FUNDS',
                'REFERENCE_NOT_PROCESSABLE',
                'REFERENCE_MISMATCH',
                'DUPLICATE_REVERSAL'
            )
        ),
    CONSTRAINT wager_transactions_external_metadata_valid
        CHECK (
            (
                kind = 'OPENING'
                AND external_transaction_id IS NULL
                AND provider_id IS NULL
                AND idempotency_key IS NULL
                AND payload_hash IS NULL
                AND round_id IS NULL
                AND game_id IS NULL
            )
            OR (
                kind <> 'OPENING'
                AND external_transaction_id IS NOT NULL
                AND BTRIM(external_transaction_id) <> ''
                AND provider_id IS NOT NULL
                AND BTRIM(provider_id) <> ''
                AND idempotency_key IS NOT NULL
                AND BTRIM(idempotency_key) <> ''
                AND payload_hash IS NOT NULL
                AND BTRIM(payload_hash) <> ''
                AND round_id IS NOT NULL
                AND BTRIM(round_id) <> ''
                AND game_id IS NOT NULL
                AND BTRIM(game_id) <> ''
            )
        ),
    CONSTRAINT wager_transactions_reference_metadata_valid
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
        ),
    CONSTRAINT wager_transactions_result_balance_valid
        CHECK (
            (
                status = 'PROCESSED'
                AND result_balance_in_cents IS NOT NULL
                AND result_balance_in_cents >= 0
                AND result_balance_currency = currency
                AND failure_code IS NULL
            )
            OR (
                status IN ('REJECTED', 'FAILED')
                AND result_balance_in_cents IS NULL
                AND result_balance_currency IS NULL
                AND failure_code IS NOT NULL
            )
            OR (
                status IN ('PENDING', 'PENDING_REFERENCE')
                AND result_balance_in_cents IS NULL
                AND result_balance_currency IS NULL
                AND failure_code IS NULL
            )
        ),
    CONSTRAINT wager_transactions_reference_schedule_valid
        CHECK (
            (
                status = 'PENDING_REFERENCE'
                AND reference_attempts >= 1
                AND next_reference_attempt_at IS NOT NULL
            )
            OR (
                status <> 'PENDING_REFERENCE'
                AND reference_attempts >= 0
                AND next_reference_attempt_at IS NULL
            )
        ),
    CONSTRAINT wager_transactions_timestamps_valid
        CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX wager_transactions_provider_external_uidx
    ON wager_transactions (provider_id, external_transaction_id)
    WHERE provider_id IS NOT NULL AND external_transaction_id IS NOT NULL;

CREATE UNIQUE INDEX wager_transactions_provider_idempotency_uidx
    ON wager_transactions (provider_id, idempotency_key)
    WHERE provider_id IS NOT NULL AND idempotency_key IS NOT NULL;

CREATE INDEX wager_transactions_wallet_idx
    ON wager_transactions (wallet_id);

CREATE INDEX wager_transactions_reference_lookup_idx
    ON wager_transactions (provider_id, reference_external_transaction_id)
    WHERE reference_external_transaction_id IS NOT NULL;

CREATE INDEX wager_transactions_pending_reference_idx
    ON wager_transactions (next_reference_attempt_at)
    WHERE status = 'PENDING_REFERENCE';

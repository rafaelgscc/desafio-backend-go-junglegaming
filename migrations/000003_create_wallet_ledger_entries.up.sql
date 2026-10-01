ALTER TABLE wager_transactions
    ADD CONSTRAINT wager_transactions_id_wallet_unique
    UNIQUE (id, wallet_id);

CREATE TABLE wallet_ledger_entries (
    id TEXT PRIMARY KEY,
    wallet_id TEXT NOT NULL,
    transaction_id TEXT NOT NULL,
    direction VARCHAR(6) NOT NULL,
    amount_in_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    balance_before_in_cents BIGINT NOT NULL,
    balance_after_in_cents BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT wallet_ledger_entries_transaction_unique
        UNIQUE (transaction_id),
    CONSTRAINT wallet_ledger_entries_transaction_wallet_fk
        FOREIGN KEY (transaction_id, wallet_id)
        REFERENCES wager_transactions (id, wallet_id),
    CONSTRAINT wallet_ledger_entries_id_not_blank
        CHECK (BTRIM(id) <> ''),
    CONSTRAINT wallet_ledger_entries_direction_valid
        CHECK (direction IN ('CREDIT', 'DEBIT')),
    CONSTRAINT wallet_ledger_entries_amount_positive
        CHECK (amount_in_cents > 0),
    CONSTRAINT wallet_ledger_entries_currency_format
        CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT wallet_ledger_entries_balances_not_negative
        CHECK (
            balance_before_in_cents >= 0
            AND balance_after_in_cents >= 0
        ),
    CONSTRAINT wallet_ledger_entries_balance_consistent
        CHECK (
            (
                direction = 'CREDIT'
                AND balance_after_in_cents = balance_before_in_cents + amount_in_cents
            )
            OR (
                direction = 'DEBIT'
                AND balance_after_in_cents = balance_before_in_cents - amount_in_cents
            )
        )
);

CREATE INDEX wallet_ledger_entries_wallet_created_idx
    ON wallet_ledger_entries (wallet_id, created_at, id);

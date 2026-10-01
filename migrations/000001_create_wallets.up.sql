CREATE TABLE wallets (
    id TEXT PRIMARY KEY,
    player_id TEXT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    balance_in_cents BIGINT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT wallets_id_not_blank
        CHECK (BTRIM(id) <> ''),
    CONSTRAINT wallets_player_id_not_blank
        CHECK (BTRIM(player_id) <> ''),
    CONSTRAINT wallets_currency_format
        CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT wallets_balance_not_negative
        CHECK (balance_in_cents >= 0),
    CONSTRAINT wallets_version_positive
        CHECK (version >= 1),
    CONSTRAINT wallets_updated_at_not_before_created_at
        CHECK (updated_at >= created_at),
    CONSTRAINT wallets_player_currency_unique
        UNIQUE (player_id, currency)
);

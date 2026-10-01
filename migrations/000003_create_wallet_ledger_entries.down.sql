DROP TABLE wallet_ledger_entries;

ALTER TABLE wager_transactions
    DROP CONSTRAINT wager_transactions_id_wallet_unique;

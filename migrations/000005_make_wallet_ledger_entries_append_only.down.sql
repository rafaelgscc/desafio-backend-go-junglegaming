DROP TRIGGER wallet_ledger_entries_reject_truncate
    ON wallet_ledger_entries;

DROP TRIGGER wallet_ledger_entries_reject_update_delete
    ON wallet_ledger_entries;

DROP FUNCTION reject_wallet_ledger_entries_mutation();

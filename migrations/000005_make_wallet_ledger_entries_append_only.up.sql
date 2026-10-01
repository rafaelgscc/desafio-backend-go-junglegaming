CREATE FUNCTION reject_wallet_ledger_entries_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'wallet ledger entries are append-only'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER wallet_ledger_entries_reject_update_delete
    BEFORE UPDATE OR DELETE ON wallet_ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION reject_wallet_ledger_entries_mutation();

CREATE TRIGGER wallet_ledger_entries_reject_truncate
    BEFORE TRUNCATE ON wallet_ledger_entries
    FOR EACH STATEMENT
    EXECUTE FUNCTION reject_wallet_ledger_entries_mutation();

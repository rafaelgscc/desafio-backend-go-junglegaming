package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestThreeInstancesPreserveBalanceAndIdempotencyUnderConcurrentBets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pools := openConcurrencyTestPools(t, ctx, "three_instances_concurrent_bets", 3)
	now := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)
	openConcurrencyWallet(t, ctx, pools[0], "wallet-1", "player-1", "100.00", now)

	money, err := domain.NewMoney("80.00", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	commands := []application.ExecuteWagerTransactionCommand{
		newConcurrentBetCommand("bet-1", "external-bet-1", "key-bet-1", "wallet-1", "player-1", money, now),
		newConcurrentBetCommand("bet-2", "external-bet-2", "key-bet-2", "wallet-1", "player-1", money, now),
	}
	executors := make([]*application.ExecuteWagerTransactionUseCase, len(pools))
	for index, pool := range pools {
		executors[index] = newPostgresWagerExecutor(t, pool)
	}

	type executionResult struct {
		commandIndex int
		result       application.ExecuteWagerTransactionResult
		err          error
	}
	start := make(chan struct{})
	results := make(chan executionResult, 3)
	for instance, commandIndex := range []int{0, 1, 0} {
		go func(executor *application.ExecuteWagerTransactionUseCase, index int) {
			<-start
			result, err := executor.Execute(ctx, commands[index])
			results <- executionResult{commandIndex: index, result: result, err: err}
		}(executors[instance], commandIndex)
	}
	close(start)
	for range 3 {
		execution := <-results
		if execution.err != nil {
			t.Fatalf("concurrent command %d failed: %v", execution.commandIndex, execution.err)
		}
	}

	var balance, version int64
	if err := pools[0].QueryRow(ctx, `
		SELECT balance_in_cents, version FROM wallets WHERE id = 'wallet-1'
	`).Scan(&balance, &version); err != nil {
		t.Fatal(err)
	}
	if balance != 2000 || version != 2 {
		t.Fatalf("wallet balance/version = %d/%d, want 2000/2", balance, version)
	}

	var processed, rejected, insufficientFunds, transactionCount, ledgerCount int
	if err := pools[0].QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PROCESSED'),
			COUNT(*) FILTER (WHERE status = 'REJECTED'),
			COUNT(*) FILTER (WHERE failure_code = 'INSUFFICIENT_FUNDS'),
			COUNT(*)
		FROM wager_transactions
		WHERE id IN ('bet-1', 'bet-2')
	`).Scan(&processed, &rejected, &insufficientFunds, &transactionCount); err != nil {
		t.Fatal(err)
	}
	if processed != 1 || rejected != 1 || insufficientFunds != 1 || transactionCount != 2 {
		t.Fatalf(
			"transactions: processed=%d rejected=%d insufficient=%d total=%d",
			processed, rejected, insufficientFunds, transactionCount,
		)
	}
	if err := pools[0].QueryRow(ctx, `
		SELECT COUNT(*) FROM wallet_ledger_entries
		WHERE wallet_id = 'wallet-1' AND direction = 'DEBIT'
	`).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 {
		t.Fatalf("ledger entries = %d, want 1", ledgerCount)
	}

	var processedID string
	if err := pools[0].QueryRow(ctx, `
		SELECT id FROM wager_transactions
		WHERE id IN ('bet-1', 'bet-2') AND status = 'PROCESSED'
	`).Scan(&processedID); err != nil {
		t.Fatal(err)
	}
	processedCommand := commands[0]
	if processedID == "bet-2" {
		processedCommand = commands[1]
	}

	const replayCount = 50
	replayErrors := make(chan error, replayCount)
	var waitGroup sync.WaitGroup
	for replay := range replayCount {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			result, err := executors[index%len(executors)].Execute(ctx, processedCommand)
			if err != nil {
				replayErrors <- err
				return
			}
			if !result.IdempotentReplay || result.Status != domain.WagerTransactionStatusProcessed ||
				!result.HasBalance || result.Balance.Amount() != "20.00" {
				replayErrors <- fmt.Errorf("unexpected replay result: %#v", result)
			}
		}(replay)
	}
	waitGroup.Wait()
	close(replayErrors)
	for err := range replayErrors {
		t.Error(err)
	}
	if t.Failed() {
		return
	}
	if err := pools[0].QueryRow(ctx, `
		SELECT balance_in_cents, version FROM wallets WHERE id = 'wallet-1'
	`).Scan(&balance, &version); err != nil {
		t.Fatal(err)
	}
	if err := pools[0].QueryRow(ctx, `
		SELECT COUNT(*) FROM wallet_ledger_entries
		WHERE wallet_id = 'wallet-1' AND direction = 'DEBIT'
	`).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if balance != 2000 || version != 2 || ledgerCount != 1 {
		t.Fatalf(
			"state changed after replays: balance=%d version=%d ledger=%d",
			balance, version, ledgerCount,
		)
	}
	assertWalletMatchesLedger(t, ctx, pools[0], "wallet-1")
}

func TestFiftyConcurrentDeliveriesProduceOneDebit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pools := openConcurrencyTestPools(t, ctx, "fifty_concurrent_deliveries", 3)
	now := time.Date(2026, time.October, 2, 12, 30, 0, 0, time.UTC)
	openConcurrencyWallet(t, ctx, pools[0], "wallet-1", "player-1", "100.00", now)
	money, _ := domain.NewMoney("25.00", "BRL")
	command := newConcurrentBetCommand(
		"bet-repeated", "external-bet-repeated", "key-bet-repeated",
		"wallet-1", "player-1", money, now,
	)
	executors := make([]*application.ExecuteWagerTransactionUseCase, len(pools))
	for index, pool := range pools {
		executors[index] = newPostgresWagerExecutor(t, pool)
	}

	const deliveries = 50
	start := make(chan struct{})
	results := make(chan application.ExecuteWagerTransactionResult, deliveries)
	errorsFound := make(chan error, deliveries)
	var waitGroup sync.WaitGroup
	for delivery := range deliveries {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			<-start
			result, err := executors[index%len(executors)].Execute(ctx, command)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- result
		}(delivery)
	}
	close(start)
	waitGroup.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent delivery failed: %v", err)
	}
	for result := range results {
		if result.Status != domain.WagerTransactionStatusProcessed ||
			!result.HasBalance || result.Balance.Amount() != "75.00" {
			t.Errorf("delivery result = %#v", result)
		}
	}
	if t.Failed() {
		return
	}

	var balance, version, transactions, debits int64
	if err := pools[0].QueryRow(ctx, `
		SELECT balance_in_cents, version FROM wallets WHERE id = 'wallet-1'
	`).Scan(&balance, &version); err != nil {
		t.Fatal(err)
	}
	if err := pools[0].QueryRow(ctx, `
		SELECT COUNT(*) FROM wager_transactions WHERE id = 'bet-repeated'
	`).Scan(&transactions); err != nil {
		t.Fatal(err)
	}
	if err := pools[0].QueryRow(ctx, `
		SELECT COUNT(*) FROM wallet_ledger_entries
		WHERE transaction_id = 'bet-repeated' AND direction = 'DEBIT'
	`).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if balance != 7500 || version != 2 || transactions != 1 || debits != 1 {
		t.Fatalf(
			"balance=%d version=%d transactions=%d debits=%d",
			balance, version, transactions, debits,
		)
	}
	assertWalletMatchesLedger(t, ctx, pools[0], "wallet-1")
}

func TestDifferentWalletsAdvanceWhileAnotherWalletIsLocked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pools := openConcurrencyTestPools(t, ctx, "independent_wallet_parallelism", 3)
	now := time.Date(2026, time.October, 2, 13, 0, 0, 0, time.UTC)
	insertConcurrencyWallet(t, ctx, pools[0], "wallet-a", "player-a", 10000, now)
	insertConcurrencyWallet(t, ctx, pools[0], "wallet-b", "player-b", 10000, now)
	insertPendingBet(t, ctx, pools[0], "bet-a", "wallet-a", "player-a", now)
	insertPendingBet(t, ctx, pools[0], "bet-b", "wallet-b", "player-b", now)

	blocker, err := pools[0].Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback(context.Background()) }()
	var lockedWallet string
	if err := blocker.QueryRow(ctx, `
		SELECT id FROM wallets WHERE id = 'wallet-a' FOR UPDATE
	`).Scan(&lockedWallet); err != nil {
		t.Fatal(err)
	}

	processA, _ := application.NewProcessBetUseCase(mustWageringUnitOfWork(t, pools[1]))
	processB, _ := application.NewProcessBetUseCase(mustWageringUnitOfWork(t, pools[2]))
	resultA := make(chan error, 1)
	go func() {
		_, err := processA.Execute(ctx, newProcessBetCommand("bet-a", now))
		resultA <- err
	}()
	waitUntilWagerRowIsLocked(t, ctx, pools[2], "bet-a")

	independentDone := make(chan error, 1)
	go func() {
		_, err := processB.Execute(ctx, newProcessBetCommand("bet-b", now))
		independentDone <- err
	}()
	select {
	case err := <-independentDone:
		if err != nil {
			t.Fatalf("independent wallet processing failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wallet-b was blocked by a lock held only on wallet-a")
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-resultA:
		if err != nil {
			t.Fatalf("wallet-a processing failed after lock release: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wallet-a did not resume after its row lock was released")
	}

	rows, err := pools[0].Query(ctx, `
		SELECT id, balance_in_cents, version FROM wallets
		WHERE id IN ('wallet-a', 'wallet-b') ORDER BY id
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var balance, version int64
		if err := rows.Scan(&id, &balance, &version); err != nil {
			t.Fatal(err)
		}
		if balance != 9000 || version != 2 {
			t.Fatalf("%s balance/version = %d/%d, want 9000/2", id, balance, version)
		}
	}
}

func openConcurrencyTestPools(
	t *testing.T,
	ctx context.Context,
	schemaName string,
	count int,
) []*pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	adminPool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL: databaseURL, MaxConnections: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}

	isolatedURL := databaseURLWithSearchPath(t, databaseURL, schemaName)
	pools := make([]*pgxpool.Pool, 0, count)
	for range count {
		pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
			DatabaseURL: isolatedURL, MaxConnections: 12,
		})
		if err != nil {
			adminPool.Close()
			t.Fatal(err)
		}
		pools = append(pools, pool)
	}
	for _, migration := range []string{
		"000001_create_wallets.up.sql",
		"000002_create_wager_transactions.up.sql",
		"000003_create_wallet_ledger_entries.up.sql",
		"000004_create_outbox_events.up.sql",
		"000005_make_wallet_ledger_entries_append_only.up.sql",
		"000008_allow_optional_win_reference.up.sql",
	} {
		if _, err := pools[0].Exec(ctx, readMigrationFile(t, migration)); err != nil {
			adminPool.Close()
			for _, pool := range pools {
				pool.Close()
			}
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}
	t.Cleanup(func() {
		for _, pool := range pools {
			pool.Close()
		}
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE")
		adminPool.Close()
	})
	return pools
}

func newPostgresWagerExecutor(
	t *testing.T,
	pool *pgxpool.Pool,
) *application.ExecuteWagerTransactionUseCase {
	t.Helper()
	unitOfWork := mustWageringUnitOfWork(t, pool)
	submit, _ := application.NewSubmitWagerTransactionUseCase(unitOfWork)
	bet, _ := application.NewProcessBetUseCase(unitOfWork)
	win, _ := application.NewProcessWinUseCase(unitOfWork)
	loss, _ := application.NewProcessLossUseCase(unitOfWork)
	refund, _ := application.NewProcessRefundUseCase(unitOfWork)
	rollback, _ := application.NewProcessRollbackUseCase(unitOfWork)
	executor, err := application.NewExecuteWagerTransactionUseCase(
		submit, bet, win, loss, refund, rollback,
	)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustWageringUnitOfWork(
	t *testing.T,
	pool *pgxpool.Pool,
) *platformpostgres.PostgresWageringUnitOfWork {
	t.Helper()
	unitOfWork, err := platformpostgres.NewPostgresWageringUnitOfWork(pool)
	if err != nil {
		t.Fatal(err)
	}
	return unitOfWork
}

func insertConcurrencyWallet(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	walletID string,
	playerID string,
	balance int64,
	now time.Time,
) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES ($1, $2, 'BRL', $3, 1, $4, $4)
	`, walletID, playerID, balance, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func openConcurrencyWallet(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	walletID string,
	playerID string,
	amount string,
	now time.Time,
) {
	t.Helper()
	balance, err := domain.NewMoney(amount, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	useCase, err := application.NewOpenWalletUseCase(mustWageringUnitOfWork(t, pool))
	if err != nil {
		t.Fatal(err)
	}
	_, err = useCase.Execute(ctx, application.OpenWalletCommand{
		WalletID: walletID, PlayerID: playerID, InitialBalance: balance,
		OpeningTransactionID:  "opening-" + walletID,
		LedgerEntryID:         "opening-ledger-" + walletID,
		ProcessedEventID:      "opening-processed-" + walletID,
		BalanceChangedEventID: "opening-balance-" + walletID,
		CorrelationID:         "opening-correlation-" + walletID,
		CreatedAt:             now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertWalletMatchesLedger(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	walletID string,
) {
	t.Helper()
	var stored, calculated int64
	if err := pool.QueryRow(ctx, `
		SELECT
			wallet.balance_in_cents,
			COALESCE(SUM(
				CASE ledger.direction
					WHEN 'CREDIT' THEN ledger.amount_in_cents
					ELSE -ledger.amount_in_cents
				END
			), 0)
		FROM wallets AS wallet
		LEFT JOIN wallet_ledger_entries AS ledger ON ledger.wallet_id = wallet.id
		WHERE wallet.id = $1
		GROUP BY wallet.id, wallet.balance_in_cents
	`, walletID).Scan(&stored, &calculated); err != nil {
		t.Fatal(err)
	}
	if stored != calculated {
		t.Fatalf("wallet %s stored balance=%d ledger balance=%d", walletID, stored, calculated)
	}
}

func insertPendingBet(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	transactionID string,
	walletID string,
	playerID string,
	now time.Time,
) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO wager_transactions (
			id, external_transaction_id, provider_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount_in_cents, currency,
			status, reference_attempts, created_at, updated_at
		) VALUES (
			$1, $2, 'provider-1', $3, $4,
			$5, $6, 'round-1', 'game-1', 'BET', 1000, 'BRL',
			'PENDING', 0, $7, $7
		)
	`, transactionID, "external-"+transactionID, "key-"+transactionID,
		"hash-"+transactionID, walletID, playerID, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func newConcurrentBetCommand(
	transactionID string,
	externalID string,
	idempotencyKey string,
	walletID string,
	playerID string,
	money domain.Money,
	now time.Time,
) application.ExecuteWagerTransactionCommand {
	return application.ExecuteWagerTransactionCommand{
		TransactionID: transactionID, LedgerEntryID: "ledger-" + transactionID,
		OutcomeEventID:        "outcome-" + transactionID,
		BalanceChangedEventID: "balance-" + transactionID,
		IdempotencyKey:        idempotencyKey, CorrelationID: "correlation-" + transactionID,
		OccurredAt: now,
		Payload: application.WagerTransactionPayload{
			ProviderID: "provider-1", ExternalTransactionID: externalID,
			PlayerID: playerID, WalletID: walletID, RoundID: "round-1",
			GameID: "game-1", Kind: domain.WagerTransactionKindBet, Money: money,
		},
	}
}

func newProcessBetCommand(transactionID string, now time.Time) application.ProcessBetCommand {
	return application.ProcessBetCommand{
		TransactionID: transactionID, LedgerEntryID: "ledger-" + transactionID,
		OutcomeEventID:        "outcome-" + transactionID,
		BalanceChangedEventID: "balance-" + transactionID,
		CorrelationID:         "correlation-" + transactionID, ProcessedAt: now,
	}
}

func waitUntilWagerRowIsLocked(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	transactionID string,
) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var id string
		err := pool.QueryRow(ctx, `
			SELECT id FROM wager_transactions WHERE id = $1 FOR UPDATE NOWAIT
		`, transactionID).Scan(&id)
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "55P03" {
			return
		}
		if err != nil {
			t.Fatalf("detect wager row lock: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("wager processing did not reach the wallet lock")
}

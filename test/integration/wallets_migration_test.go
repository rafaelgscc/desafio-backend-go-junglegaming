package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestWalletsMigrationUpAndDown(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:    databaseURL,
		MaxConnections: 1,
	})
	if err != nil {
		t.Fatalf("OpenPool() unexpected error: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}
	defer conn.Release()

	schemaName := "wallets_migration_test"
	quotedSchemaName := pgx.Identifier{schemaName}.Sanitize()

	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE"); err != nil {
		t.Fatalf("drop test schema before test: %v", err)
	}
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+quotedSchemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchemaName+" CASCADE")
	}()

	if _, err := conn.Exec(ctx, "SET search_path TO "+quotedSchemaName); err != nil {
		t.Fatalf("set test search_path: %v", err)
	}

	upSQL := readMigrationFile(t, "000001_create_wallets.up.sql")
	if _, err := conn.Exec(ctx, upSQL); err != nil {
		t.Fatalf("apply wallets up migration: %v", err)
	}

	var tableName string
	if err := conn.QueryRow(ctx, "SELECT to_regclass('wallets')::text").Scan(&tableName); err != nil {
		t.Fatalf("find wallets table after up migration: %v", err)
	}
	if tableName != "wallets" {
		t.Fatalf("wallets table = %q, want %q", tableName, "wallets")
	}

	_, err = conn.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES (
			'wallet-1', 'player-1', 'BRL', 10000, 1, NOW(), NOW()
		)
	`)
	if err != nil {
		t.Fatalf("insert valid wallet: %v", err)
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO wallets (
			id, player_id, currency, balance_in_cents, version, created_at, updated_at
		) VALUES (
			'wallet-2', 'player-2', 'BRL', -1, 1, NOW(), NOW()
		)
	`); err == nil {
		t.Fatal("insert wallet with negative balance unexpectedly succeeded")
	}

	downSQL := readMigrationFile(t, "000001_create_wallets.down.sql")
	if _, err := conn.Exec(ctx, downSQL); err != nil {
		t.Fatalf("apply wallets down migration: %v", err)
	}

	var removedTableName *string
	if err := conn.QueryRow(ctx, "SELECT to_regclass('wallets')::text").Scan(&removedTableName); err != nil {
		t.Fatalf("find wallets table after down migration: %v", err)
	}
	if removedTableName != nil {
		t.Fatalf("wallets table still exists after down migration: %q", *removedTableName)
	}
}

func readMigrationFile(t *testing.T, name string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate integration test file")
	}

	path := filepath.Join(filepath.Dir(currentFile), "..", "..", "migrations", name)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}

	return string(contents)
}

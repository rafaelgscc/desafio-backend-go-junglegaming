package postgres

import (
	"context"
	"errors"
	"testing"
)

func TestOpenPoolRejectsEmptyDatabaseURL(t *testing.T) {
	t.Parallel()

	pool, err := OpenPool(context.Background(), PoolConfig{})
	if pool != nil {
		pool.Close()
		t.Fatal("OpenPool() returned a pool for an empty database URL")
	}
	if !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("OpenPool() error = %v, want ErrDatabaseURLRequired", err)
	}
}

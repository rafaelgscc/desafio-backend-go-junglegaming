package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDatabaseURLRequired = errors.New("database URL is required")

type PoolConfig struct {
	DatabaseURL       string
	MaxConnections    int32
	MinConnections    int32
	MaxConnectionIdle time.Duration
	MaxConnectionLife time.Duration
}

func OpenPool(ctx context.Context, config PoolConfig) (*pgxpool.Pool, error) {
	if config.DatabaseURL == "" {
		return nil, ErrDatabaseURLRequired
	}

	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: %w", err)
	}
	if config.MaxConnections > 0 {
		poolConfig.MaxConns = config.MaxConnections
	}
	if config.MinConnections > 0 {
		poolConfig.MinConns = config.MinConnections
	}
	if config.MaxConnectionIdle > 0 {
		poolConfig.MaxConnIdleTime = config.MaxConnectionIdle
	}
	if config.MaxConnectionLife > 0 {
		poolConfig.MaxConnLifetime = config.MaxConnectionLife
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return pool, nil
}

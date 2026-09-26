// Package postgres owns the PostgreSQL connection pool and schema migrations.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const connectTimeout = 5 * time.Second

// poolConfig parses the URL and bounds every connection attempt. pgx keeps
// building a connection in the background after the caller's context is
// cancelled, and closing the pool waits for it; without a bound, a dial to an
// unreachable database could stall shutdown until the OS gives up. A
// connect_timeout in the URL takes precedence.
func poolConfig(databaseURL string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = connectTimeout
	}
	return cfg, nil
}

// Connect creates a connection pool and verifies that PostgreSQL is reachable.
// Errors never include the password (pgx redacts it when parsing fails).
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := poolConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL at %s:%d/%s: %w",
			cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database, err)
	}
	return pool, nil
}

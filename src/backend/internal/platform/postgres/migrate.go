package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

// Migrator applies the goose migrations in database/migrations, the single
// source of truth for the schema.
type Migrator struct {
	provider *goose.Provider
	db       *sql.DB
}

// NewMigrator prepares migrations from dir against the database behind pool.
// Close releases the database/sql handle; the pool stays open.
func NewMigrator(pool *pgxpool.Pool, dir string) (*Migrator, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("migrations directory: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(database.DialectPostgres, db, os.DirFS(dir))
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load migrations from %s: %w", dir, err)
	}
	return &Migrator{provider: provider, db: db}, nil
}

// Up applies all pending migrations and returns the versions applied.
func (m *Migrator) Up(ctx context.Context) ([]int64, error) {
	results, err := m.provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return versions(results), nil
}

// Down rolls back the most recently applied migration and returns its version.
func (m *Migrator) Down(ctx context.Context) (int64, error) {
	result, err := m.provider.Down(ctx)
	if err != nil {
		return 0, fmt.Errorf("roll back migration: %w", err)
	}
	return result.Source.Version, nil
}

// Version returns the current schema version (0 when nothing is applied).
func (m *Migrator) Version(ctx context.Context) (int64, error) {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return v, nil
}

// Status reports every known migration and whether it is applied.
func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	status, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("read migration status: %w", err)
	}
	return status, nil
}

// Close releases the database/sql handle used by goose.
func (m *Migrator) Close() error {
	return m.db.Close()
}

func versions(results []*goose.MigrationResult) []int64 {
	out := make([]int64, 0, len(results))
	for _, r := range results {
		out = append(out, r.Source.Version)
	}
	return out
}

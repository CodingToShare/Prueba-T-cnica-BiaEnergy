package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bia-energy.local/backend/internal/platform/postgres/dbgen"
)

// ReadSnapshot runs fn in a read-only REPEATABLE READ transaction, so all of
// a request's queries (for example a page and its total, or the current run
// and its findings) see the same committed state even if a run completes
// meanwhile.
func ReadSnapshot(ctx context.Context, pool *pgxpool.Pool, fn func(q *dbgen.Queries) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin read snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(dbgen.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

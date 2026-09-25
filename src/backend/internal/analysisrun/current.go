package analysisrun

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"bia-energy.local/backend/internal/platform/postgres/dbgen"
)

// LatestCompleted returns the current analytical state, or nil when no run
// has completed yet.
func LatestCompleted(ctx context.Context, q *dbgen.Queries) (*CompletedRun, error) {
	row, err := q.GetLatestCompletedRunSummary(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read latest completed analysis run: %w", err)
	}
	c := &CompletedRun{ID: row.ID, AggregateConfidence: row.AggregateConfidence}
	if row.CompletedAt != nil {
		c.CompletedAt = *row.CompletedAt
	}
	if row.FindingsCount != nil {
		c.FindingsCount = int(*row.FindingsCount)
	}
	if row.HighPriorityCount != nil {
		c.HighPriorityCount = int(*row.HighPriorityCount)
	}
	return c, nil
}

// Active returns the QUEUED or RUNNING run, or nil.
func Active(ctx context.Context, q *dbgen.Queries) (*Run, error) {
	row, err := q.GetActiveAnalysisRun(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read active analysis run: %w", err)
	}
	r := runOf(row)
	return &r, nil
}

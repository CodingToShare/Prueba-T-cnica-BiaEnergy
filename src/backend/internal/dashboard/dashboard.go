// Package dashboard serves the fleet summary. Source figures (meters,
// consumption, period) come from the source data; analytical figures come
// only from the latest COMPLETED run and are absent before one exists.
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/platform/postgres"
	"bia-energy.local/backend/internal/platform/postgres/dbgen"
)

// Summary is the dashboard content.
type Summary struct {
	Meters              int64
	Readings            int64
	TotalConsumptionKWh float64
	FirstReadingAt      *time.Time
	LastReadingAt       *time.Time
	// Anomalies and HighPriority (severity HIGH) count the findings of the
	// latest completed run; 0 before any run completes.
	Anomalies    int
	HighPriority int
	// AggregateConfidence is the mean confidence of those findings; nil when
	// no run has completed or the run has no findings.
	AggregateConfidence *float64
	LatestAnalysis      *analysisrun.CompletedRun
	ActiveAnalysis      *analysisrun.Run
}

// Service reads the summary.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns a dashboard service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Summary reads every figure from one consistent snapshot.
func (s *Service) Summary(ctx context.Context) (Summary, error) {
	var sum Summary
	err := postgres.ReadSnapshot(ctx, s.pool, func(q *dbgen.Queries) error {
		totals, err := q.GetSourceTotals(ctx)
		if err != nil {
			return fmt.Errorf("read source totals: %w", err)
		}
		sum.Meters, sum.Readings, sum.TotalConsumptionKWh = totals.MetersCount, totals.ReadingsCount, totals.TotalConsumptionKwh
		period, err := q.GetReadingPeriod(ctx, nil)
		switch {
		case err == nil:
			sum.FirstReadingAt, sum.LastReadingAt = &period.FirstReadingAt, &period.LastReadingAt
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("read reading period: %w", err)
		}
		if sum.LatestAnalysis, err = analysisrun.LatestCompleted(ctx, q); err != nil {
			return err
		}
		if l := sum.LatestAnalysis; l != nil {
			sum.Anomalies, sum.HighPriority, sum.AggregateConfidence = l.FindingsCount, l.HighPriorityCount, l.AggregateConfidence
		}
		sum.ActiveAnalysis, err = analysisrun.Active(ctx, q)
		return err
	})
	return sum, err
}

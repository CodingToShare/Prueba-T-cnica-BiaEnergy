// Package meter serves the meter list, meter detail and reading history.
// Analytical values (status, variation, finding) come from the latest
// COMPLETED analysis run; source_status is only ever the CSV status column.
package meter

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

// ErrNotFound reports an unknown meter.
var ErrNotFound = errors.New("meter not found")

// SortKey selects the list order; only these values reach the query.
type SortKey string

// Sort keys.
const (
	SortMeterID     SortKey = "meter_id"
	SortConsumption SortKey = "consumption"
	SortVariation   SortKey = "variation"
	SortSeverity    SortKey = "severity"
)

// ListParams filter, sort and page the meter list. Pointers are optional.
type ListParams struct {
	Search         *string
	ComputedStatus *string
	Sort           SortKey
	Descending     bool
	Limit          int
	Offset         int
}

// Summary is one row of the meter list. Analytical fields are nil when the
// meter has no finding in the current run (or no run has completed).
type Summary struct {
	MeterID             string
	TotalConsumptionKWh float64
	ComputedStatus      *string
	AnomalyID           *int64
	AnomalyType         *string
	Severity            *string
	Confidence          *float64
	Priority            *int
	VariationPct        *float64
}

// Page is a page of meters with the total number of matches.
type Page struct {
	Items    []Summary
	Total    int64
	Analysis *analysisrun.CompletedRun
}

// Finding is a meter's finding in the current run.
type Finding struct {
	AnomalyID         int64
	Type              string
	Severity          string
	Confidence        float64
	Priority          int
	VariationPct      float64
	StartedAt         time.Time
	LastObservedAt    time.Time
	RecommendedAction string
	Reason            string
}

// Detail is the meter detail.
type Detail struct {
	MeterID             string
	TotalConsumptionKWh float64
	ReadingsCount       int64
	FirstReadingAt      *time.Time
	LastReadingAt       *time.Time
	ComputedStatus      *string
	Analysis            *analysisrun.CompletedRun
	// CurrentFinding is the highest-priority finding; FindingsCount counts
	// all of the meter's findings in the current run.
	CurrentFinding *Finding
	FindingsCount  int
}

// Reading is one source measurement.
type Reading struct {
	Timestamp      time.Time
	ConsumptionKWh float64
	VoltageV       float64
	CurrentA       float64
	PowerFactor    float64
	SourceStatus   string
}

// ReadingsParams bound the history: [From, To) and at most Limit rows.
type ReadingsParams struct {
	From  *time.Time
	To    *time.Time
	Limit int
}

// Service reads meters.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns a meter service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// List returns one page of meters, with every aggregate in one query.
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	var page Page
	err := postgres.ReadSnapshot(ctx, s.pool, func(q *dbgen.Queries) error {
		rows, err := q.ListMeters(ctx, dbgen.ListMetersParams{
			Search: p.Search, ComputedStatus: p.ComputedStatus,
			SortKey: string(p.Sort), SortDesc: p.Descending,
			RowLimit: int32(p.Limit), RowOffset: int32(p.Offset),
		})
		if err != nil {
			return fmt.Errorf("list meters: %w", err)
		}
		if page.Total, err = q.CountMeters(ctx, dbgen.CountMetersParams{Search: p.Search, ComputedStatus: p.ComputedStatus}); err != nil {
			return fmt.Errorf("count meters: %w", err)
		}
		if page.Analysis, err = analysisrun.LatestCompleted(ctx, q); err != nil {
			return err
		}
		page.Items = make([]Summary, len(rows))
		for i, r := range rows {
			page.Items[i] = Summary{
				MeterID: r.MeterID, TotalConsumptionKWh: r.TotalConsumptionKwh, ComputedStatus: r.ComputedStatus,
				AnomalyID: r.AnomalyID, AnomalyType: r.AnomalyType, Severity: r.Severity,
				Confidence: r.Confidence, Priority: intPtr(r.Priority), VariationPct: r.VariationPct,
			}
		}
		return nil
	})
	return page, err
}

// Get returns a meter's detail, or ErrNotFound.
func (s *Service) Get(ctx context.Context, meterID string) (Detail, error) {
	var d Detail
	err := postgres.ReadSnapshot(ctx, s.pool, func(q *dbgen.Queries) error {
		row, err := q.GetMeterOverview(ctx, meterID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("read meter %s: %w", meterID, err)
		}
		d = Detail{
			MeterID: row.MeterID, TotalConsumptionKWh: row.TotalConsumptionKwh,
			ReadingsCount: row.ReadingsCount, ComputedStatus: row.ComputedStatus,
		}
		mid := meterID
		period, err := q.GetReadingPeriod(ctx, &mid)
		switch {
		case err == nil:
			d.FirstReadingAt, d.LastReadingAt = &period.FirstReadingAt, &period.LastReadingAt
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("read reading period of meter %s: %w", meterID, err)
		}
		if d.Analysis, err = analysisrun.LatestCompleted(ctx, q); err != nil {
			return err
		}
		findings, err := q.ListCurrentMeterFindings(ctx, meterID)
		if err != nil {
			return fmt.Errorf("list findings of meter %s: %w", meterID, err)
		}
		d.FindingsCount = len(findings)
		if len(findings) > 0 {
			f := findings[0]
			d.CurrentFinding = &Finding{
				AnomalyID: f.ID, Type: f.Type, Severity: f.Severity, Confidence: f.Confidence,
				Priority: int(f.Priority), VariationPct: f.ConsumptionDeviationPct,
				StartedAt: f.StartedAt, LastObservedAt: f.LastObservedAt,
				RecommendedAction: f.RecommendedAction, Reason: f.Reason,
			}
		}
		return nil
	})
	return d, err
}

// Readings returns up to p.Limit readings of a meter in time order and
// whether more readings exist in the range. It returns ErrNotFound for an
// unknown meter.
func (s *Service) Readings(ctx context.Context, meterID string, p ReadingsParams) ([]Reading, bool, error) {
	var out []Reading
	more := false
	err := postgres.ReadSnapshot(ctx, s.pool, func(q *dbgen.Queries) error {
		exists, err := q.MeterExists(ctx, meterID)
		if err != nil {
			return fmt.Errorf("check meter %s: %w", meterID, err)
		}
		if !exists {
			return ErrNotFound
		}
		rows, err := q.ListMeterReadings(ctx, dbgen.ListMeterReadingsParams{
			MeterID: meterID, FromTs: p.From, ToTs: p.To, RowLimit: int32(p.Limit + 1),
		})
		if err != nil {
			return fmt.Errorf("list readings of meter %s: %w", meterID, err)
		}
		if len(rows) > p.Limit {
			rows, more = rows[:p.Limit], true
		}
		out = make([]Reading, len(rows))
		for i, r := range rows {
			out[i] = Reading{
				Timestamp: r.ReadingTimestamp, ConsumptionKWh: r.ConsumptionKwh, VoltageV: r.VoltageV,
				CurrentA: r.CurrentA, PowerFactor: r.PowerFactor, SourceStatus: r.SourceStatus,
			}
		}
		return nil
	})
	return out, more, err
}

func intPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}

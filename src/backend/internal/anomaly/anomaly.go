// Package anomaly serves the persisted findings. Nothing is recomputed: the
// list and detail read what the analysis run stored, including its evidence.
package anomaly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/platform/postgres"
	"bia-energy.local/backend/internal/platform/postgres/dbgen"
)

// ErrNotFound reports an unknown anomaly.
var ErrNotFound = errors.New("anomaly not found")

// ListParams filter and page the current anomalies. Pointers are optional.
type ListParams struct {
	MeterID  *string
	Type     *string
	Severity *string
	Limit    int
	Offset   int
}

// Summary is one persisted finding without its evidence.
type Summary struct {
	ID                int64
	AnalysisID        int64
	MeterID           string
	Priority          int
	Type              string
	Severity          string
	Confidence        float64
	Status            string
	StartedAt         time.Time
	LastObservedAt    time.Time
	DurationSeconds   int64
	VariationPct      float64
	Reason            string
	RecommendedAction string
}

// Page is a page of the current run's findings in priority order.
type Page struct {
	Items    []Summary
	Total    int64
	Analysis *analysisrun.CompletedRun
}

// Detail is a finding with its structured evidence and the explanation
// stored by its run (nil for findings stored before explanations existed).
type Detail struct {
	Summary
	Rule        string
	Evidence    analysisrun.Evidence
	Explanation *Explanation
	CreatedAt   time.Time
}

// Explanation is the persisted explanation of a finding with its provenance.
// The sanitized fallback code stays internal (database and logs).
type Explanation struct {
	analysisrun.ExplanationText
	Source        string
	Model         *string
	PromptVersion string
	GeneratedAt   time.Time
	FallbackUsed  bool
}

// Service reads anomalies.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns an anomaly service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// List returns the latest completed run's findings, ordered by priority.
// Before any run completes the page is empty and Analysis is nil.
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	var page Page
	err := postgres.ReadSnapshot(ctx, s.pool, func(q *dbgen.Queries) error {
		rows, err := q.ListCurrentAnomalies(ctx, dbgen.ListCurrentAnomaliesParams{
			MeterID: p.MeterID, Type: p.Type, Severity: p.Severity,
			RowLimit: int32(p.Limit), RowOffset: int32(p.Offset),
		})
		if err != nil {
			return fmt.Errorf("list anomalies: %w", err)
		}
		if page.Total, err = q.CountCurrentAnomalies(ctx, dbgen.CountCurrentAnomaliesParams{
			MeterID: p.MeterID, Type: p.Type, Severity: p.Severity,
		}); err != nil {
			return fmt.Errorf("count anomalies: %w", err)
		}
		if page.Analysis, err = analysisrun.LatestCompleted(ctx, q); err != nil {
			return err
		}
		page.Items = make([]Summary, len(rows))
		for i, r := range rows {
			page.Items[i] = Summary{
				ID: r.ID, AnalysisID: r.AnalysisRunID, MeterID: r.MeterID, Priority: int(r.Priority),
				Type: r.Type, Severity: r.Severity, Confidence: r.Confidence, Status: r.Status,
				StartedAt: r.StartedAt, LastObservedAt: r.LastObservedAt, DurationSeconds: r.DurationSeconds,
				VariationPct: r.ConsumptionDeviationPct, Reason: r.Reason, RecommendedAction: r.RecommendedAction,
			}
		}
		return nil
	})
	return page, err
}

// Get returns a finding with its evidence, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id int64) (Detail, error) {
	row, err := dbgen.New(s.pool).GetAnomaly(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, fmt.Errorf("read anomaly %d: %w", id, err)
	}
	d := Detail{
		Summary: Summary{
			ID: row.ID, AnalysisID: row.AnalysisRunID, MeterID: row.MeterID, Priority: int(row.Priority),
			Type: row.Type, Severity: row.Severity, Confidence: row.Confidence, Status: row.Status,
			StartedAt: row.StartedAt, LastObservedAt: row.LastObservedAt, DurationSeconds: row.DurationSeconds,
			VariationPct: row.ConsumptionDeviationPct, Reason: row.Reason, RecommendedAction: row.RecommendedAction,
		},
		Rule:      row.Rule,
		CreatedAt: row.CreatedAt,
	}
	if err := json.Unmarshal(row.Evidence, &d.Evidence); err != nil {
		return Detail{}, fmt.Errorf("decode evidence of anomaly %d: %w", id, err)
	}
	if row.Explanation != nil {
		// The table constraint anomalies_explanation_complete guarantees the
		// provenance columns are set together with the explanation.
		e := Explanation{
			Source: derefString(row.ExplanationSource), Model: row.ExplanationModel,
			PromptVersion: derefString(row.ExplanationPromptVersion),
			FallbackUsed:  row.ExplanationFallbackUsed != nil && *row.ExplanationFallbackUsed,
		}
		if row.ExplanationGeneratedAt != nil {
			e.GeneratedAt = *row.ExplanationGeneratedAt
		}
		if err := json.Unmarshal(row.Explanation, &e.ExplanationText); err != nil {
			return Detail{}, fmt.Errorf("decode explanation of anomaly %d: %w", id, err)
		}
		d.Explanation = &e
	}
	return d, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

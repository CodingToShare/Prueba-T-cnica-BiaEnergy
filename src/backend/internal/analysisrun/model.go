// Package analysisrun orchestrates analysis runs (ADR-007, ADR-009): it
// persists each run in PostgreSQL, executes queued runs in the background
// with the deterministic engine (internal/analysis), and stores the findings
// atomically. The engine itself stays free of persistence.
package analysisrun

import (
	"context"
	"errors"
	"time"

	"bia-energy.local/backend/internal/analysis"
)

// Status is the lifecycle state of a run.
type Status string

// Run statuses.
const (
	StatusQueued    Status = "QUEUED"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

// Stage is the step a run is in. Stages change only when the work actually
// moves on; there are no simulated delays or intermediate percentages.
type Stage string

// Run stages.
const (
	StageQueued            Stage = "QUEUED"
	StageLoadingData       Stage = "LOADING_DATA"
	StageAnalyzing         Stage = "ANALYZING"
	StagePersistingResults Stage = "PERSISTING_RESULTS"
	StageCompleted         Stage = "COMPLETED"
	StageFailed            Stage = "FAILED"
)

// Progress is the fixed percentage reported when a stage starts. FAILED
// keeps the progress of the stage that failed.
func Progress(s Stage) int {
	switch s {
	case StageLoadingData:
		return 10
	case StageAnalyzing:
		return 35
	case StagePersistingResults:
		return 85
	case StageCompleted:
		return 100
	default:
		return 0
	}
}

// Failure codes stored on FAILED runs. Messages are fixed, safe texts.
const (
	CodeLoadFailed        = "load_failed"
	CodeAnalysisFailed    = "analysis_failed"
	CodePersistenceFailed = "persistence_failed"
	CodeTimeout           = "timeout"
	CodeInterrupted       = "interrupted"
)

var failureMessages = map[string]string{
	CodeLoadFailed:        "The source data could not be loaded.",
	CodeAnalysisFailed:    "The analysis engine could not analyze the source data.",
	CodePersistenceFailed: "The analysis results could not be saved.",
	CodeTimeout:           "The analysis did not finish within its time limit.",
	CodeInterrupted:       "The analysis was interrupted because the service stopped.",
}

// FailureMessage returns the safe message of a failure code.
func FailureMessage(code string) string { return failureMessages[code] }

// ErrNotFound reports an unknown run.
var ErrNotFound = errors.New("analysis run not found")

// Analyzer is the boundary between orchestration and the deterministic
// engine; *analysis.Engine implements it. Tests use it to inject failures.
type Analyzer interface {
	Analyze(ctx context.Context, readings []analysis.Reading, events []analysis.Event) (analysis.Result, error)
}

// Run is the persisted state of one analysis.
type Run struct {
	ID                  int64
	Status              Status
	Stage               Stage
	Progress            int
	EngineVersion       string
	MetersCount         *int
	ReadingsCount       *int
	EventsCount         *int
	FindingsCount       *int
	HighPriorityCount   *int
	AggregateConfidence *float64
	ErrorCode           *string
	ErrorMessage        *string
	CreatedAt           time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
}

// CompletedRun summarizes the latest COMPLETED run: the "current" analytical
// state for meters, anomalies and the dashboard. RUNNING and FAILED runs
// never replace it.
type CompletedRun struct {
	ID                  int64
	CompletedAt         time.Time
	FindingsCount       int
	HighPriorityCount   int
	AggregateConfidence *float64
}

package analysisrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/platform/postgres"
	"bia-energy.local/backend/internal/platform/postgres/dbgen"
)

// Defaults for Options.
const (
	// DefaultRunTimeout bounds one run. The supplied dataset is analyzed in
	// milliseconds; two minutes leaves ample room for slower machines and
	// larger local datasets without letting a stuck run block the queue.
	DefaultRunTimeout = 2 * time.Minute
	// DefaultPollInterval is how often the worker looks for queued runs
	// without a wake-up signal (e.g. runs queued before a restart).
	DefaultPollInterval = 10 * time.Second
	// failureRecordTimeout bounds recording a failure after cancellation.
	failureRecordTimeout = 5 * time.Second
)

// Options tune the worker. Zero values select the defaults.
type Options struct {
	RunTimeout   time.Duration
	PollInterval time.Duration
	// ExplanationTimeout is an extra budget for the explanation stage, added
	// to the run timeout. Zero means explanations share the run timeout (the
	// deterministic provider needs microseconds); a generative provider gets
	// a dedicated budget so that a slow model degrades to the fallback
	// instead of timing out the whole run.
	ExplanationTimeout time.Duration
}

// Service creates, reads and executes analysis runs. PostgreSQL is the
// source of truth for pending work; the in-memory wake-up signal only
// shortens the wait.
type Service struct {
	pool          *pgxpool.Pool
	queries       *dbgen.Queries
	analyzer      Analyzer
	explainers    Explainers
	engineVersion string
	configuration []byte
	// explanationConfiguration is the JSON of explainers.Settings.
	explanationConfiguration []byte
	logger                   *slog.Logger
	runTimeout               time.Duration
	explanationTimeout       time.Duration
	pollInterval             time.Duration
	wake                     chan struct{}
}

// NewService prepares the orchestration. configuration is the JSON snapshot
// of the engine configuration stored with every run; explainers produce the
// explanation of each finding.
func NewService(pool *pgxpool.Pool, analyzer Analyzer, explainers Explainers, engineVersion string, configuration any, logger *slog.Logger, opts Options) (*Service, error) {
	snapshot, err := json.Marshal(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode analysis configuration snapshot: %w", err)
	}
	if explainers.Primary == nil {
		return nil, errors.New("an explanation provider is required")
	}
	explanationSnapshot, err := json.Marshal(explainers.Settings)
	if err != nil {
		return nil, fmt.Errorf("encode explanation configuration snapshot: %w", err)
	}
	if opts.RunTimeout <= 0 {
		opts.RunTimeout = DefaultRunTimeout
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = DefaultPollInterval
	}
	return &Service{
		pool:          pool,
		queries:       dbgen.New(pool),
		analyzer:      analyzer,
		explainers:    explainers,
		engineVersion: engineVersion,
		configuration: snapshot,

		explanationConfiguration: explanationSnapshot,
		logger:                   logger,
		runTimeout:               opts.RunTimeout,
		explanationTimeout:       max(opts.ExplanationTimeout, 0),
		pollInterval:             opts.PollInterval,
		wake:                     make(chan struct{}, 1),
	}, nil
}

// Request queues a new run, or returns the active one. The partial unique
// index analysis_runs_single_active guarantees at most one QUEUED or RUNNING
// run even when requests race; created reports whether this call queued it.
func (s *Service) Request(ctx context.Context) (run Run, created bool, err error) {
	for range 3 {
		row, err := s.queries.CreateAnalysisRun(ctx, dbgen.CreateAnalysisRunParams{
			EngineVersion: s.engineVersion,
			Configuration: s.configuration,

			ExplanationConfiguration: s.explanationConfiguration,
		})
		if err == nil {
			s.logger.InfoContext(ctx, "analysis queued", "analysis_id", row.ID)
			s.Wake()
			return runOf(row), true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Run{}, false, fmt.Errorf("create analysis run: %w", err)
		}
		active, err := s.queries.GetActiveAnalysisRun(ctx)
		if err == nil {
			return runOf(active), false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Run{}, false, fmt.Errorf("read active analysis run: %w", err)
		}
		// The active run finished between both statements: try again.
	}
	return Run{}, false, errors.New("could not queue or find an active analysis run")
}

// Get returns a run by ID, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id int64) (Run, error) {
	row, err := s.queries.GetAnalysisRun(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("read analysis run %d: %w", id, err)
	}
	return runOf(row), nil
}

// RecoverInterrupted marks runs left RUNNING by a stopped process as FAILED
// ("interrupted"). QUEUED runs are kept and processed normally. It assumes
// a single API process, as ADR-009 records.
func (s *Service) RecoverInterrupted(ctx context.Context) ([]int64, error) {
	code, message := CodeInterrupted, FailureMessage(CodeInterrupted)
	ids, err := s.queries.FailInterruptedAnalysisRuns(ctx, dbgen.FailInterruptedAnalysisRunsParams{
		ErrorCode: &code, ErrorMessage: &message,
	})
	if err != nil {
		return nil, fmt.Errorf("recover interrupted analysis runs: %w", err)
	}
	for _, id := range ids {
		s.logger.WarnContext(ctx, "analysis failed", "analysis_id", id, "error_code", code, "reason", "interrupted by a previous shutdown")
	}
	return ids, nil
}

// Wake signals the worker that a run was queued. It never blocks.
func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Work processes queued runs one at a time until ctx is cancelled. It is
// woken by Wake and also polls, so runs queued before a restart are picked up.
func (s *Service) Work(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		for ctx.Err() == nil {
			processed, err := s.ProcessNext(ctx)
			if err != nil {
				s.logger.WarnContext(ctx, "claiming a queued analysis failed", "error", err)
				break
			}
			if !processed {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}

// ProcessNext claims the oldest queued run, if any, and executes it. The
// claim is atomic (FOR UPDATE SKIP LOCKED), so competing workers never run
// the same analysis. It reports whether a run was processed; the outcome of
// the run itself is persisted, not returned.
func (s *Service) ProcessNext(ctx context.Context) (bool, error) {
	// A queued request may survive a deployment. Record the policy that
	// actually executes it, atomically with the claim.
	row, err := s.queries.ClaimQueuedAnalysisRun(ctx, dbgen.ClaimQueuedAnalysisRunParams{
		ProgressPercent: int16(Progress(StageLoadingData)),
		EngineVersion:   s.engineVersion, Configuration: s.configuration,

		ExplanationConfiguration: s.explanationConfiguration,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim queued analysis run: %w", err)
	}
	s.execute(ctx, row.ID)
	return true, nil
}

// stageError attaches the failure code of the stage that failed.
type stageError struct {
	code string
	err  error
}

func (e *stageError) Error() string { return e.code + ": " + e.err.Error() }
func (e *stageError) Unwrap() error { return e.err }

func failAt(code string, err error) error { return &stageError{code: code, err: err} }

var errNotRunning = errors.New("the run is no longer RUNNING")

func (s *Service) execute(ctx context.Context, id int64) {
	start := time.Now()
	log := s.logger.With("analysis_id", id)
	log.InfoContext(ctx, "analysis started", "stage", StageLoadingData)

	runCtx, cancel := context.WithTimeout(ctx, s.runTimeout+s.explanationTimeout)
	defer cancel()
	findings, err := s.perform(runCtx, id, log)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		code := failureCode(ctx, runCtx, err)
		if ferr := s.recordFailure(ctx, id, code); ferr != nil {
			log.ErrorContext(ctx, "recording the analysis failure failed", "error", ferr)
		}
		log.ErrorContext(ctx, "analysis failed", "error_code", code, "duration_ms", elapsed, "error", err)
		return
	}
	log.InfoContext(ctx, "analysis completed", "findings", findings, "duration_ms", elapsed)
}

// failureCode classifies an error: shutdown and the run timeout take
// precedence over the stage in which they surfaced.
func failureCode(parent, runCtx context.Context, err error) string {
	var se *stageError
	switch {
	case parent.Err() != nil:
		return CodeInterrupted
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return CodeTimeout
	case errors.As(err, &se):
		return se.code
	default:
		return CodeAnalysisFailed
	}
}

// recordFailure stores the safe code and message. It runs even after the
// run's context was cancelled; if it cannot, startup recovery marks the run.
func (s *Service) recordFailure(ctx context.Context, id int64, code string) error {
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureRecordTimeout)
	defer cancel()
	message := FailureMessage(code)
	_, err := s.queries.FailAnalysisRun(fctx, dbgen.FailAnalysisRunParams{ID: id, ErrorCode: &code, ErrorMessage: &message})
	return err
}

func (s *Service) perform(ctx context.Context, id int64, log *slog.Logger) (int, error) {
	readings, events, meters, err := s.load(ctx)
	if err != nil {
		return 0, failAt(CodeLoadFailed, err)
	}
	n, err := s.queries.SetAnalysisRunSourceCounts(ctx, dbgen.SetAnalysisRunSourceCountsParams{
		ID: id, MetersCount: int32Ptr(meters), ReadingsCount: int32Ptr(len(readings)), EventsCount: int32Ptr(len(events)),
	})
	if err = affectedOne(n, err); err != nil {
		return 0, failAt(CodeLoadFailed, err)
	}
	log.InfoContext(ctx, "source data loaded", "meters", meters, "readings", len(readings), "events", len(events))

	if err := s.setStage(ctx, id, StageAnalyzing); err != nil {
		return 0, failAt(CodeAnalysisFailed, err)
	}
	engineStart := time.Now()
	result, err := s.analyzer.Analyze(ctx, readings, events)
	if err != nil {
		return 0, failAt(CodeAnalysisFailed, err)
	}
	log.InfoContext(ctx, "engine completed", "findings", len(result.Findings), "duration_ms", time.Since(engineStart).Milliseconds())

	evidence := make([]Evidence, len(result.Findings))
	for i, f := range result.Findings {
		evidence[i] = EvidenceOf(f)
	}
	var explanations []*storedExplanation
	if len(result.Findings) > 0 {
		if err := s.setStage(ctx, id, StageGeneratingExplanations); err != nil {
			return 0, failAt(CodeAnalysisFailed, err)
		}
		// Only the run's own cancellation (shutdown or run timeout) stops this
		// stage; provider failures fall back to the deterministic text.
		if explanations, err = s.explain(ctx, result.Findings, evidence, log); err != nil {
			return 0, failAt(CodeAnalysisFailed, err)
		}
	}

	if err := s.setStage(ctx, id, StagePersistingResults); err != nil {
		return 0, failAt(CodePersistenceFailed, err)
	}
	if err := s.persist(ctx, id, result, evidence, explanations); err != nil {
		return 0, failAt(CodePersistenceFailed, err)
	}
	log.InfoContext(ctx, "findings persisted", "findings", len(result.Findings))
	return len(result.Findings), nil
}

func (s *Service) setStage(ctx context.Context, id int64, stage Stage) error {
	n, err := s.queries.SetAnalysisRunStage(ctx, dbgen.SetAnalysisRunStageParams{
		ID: id, Stage: string(stage), ProgressPercent: int16(Progress(stage)),
	})
	return affectedOne(n, err)
}

func affectedOne(n int64, err error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return errNotRunning
	}
	return nil
}

// load reads the engine input from PostgreSQL (never from the CSV files).
func (s *Service) load(ctx context.Context) ([]analysis.Reading, []analysis.Event, int, error) {
	var rows []dbgen.ListAnalysisReadingsRow
	var eventRows []dbgen.ListAnalysisEventsRow
	err := postgres.ReadSnapshot(ctx, s.pool, func(q *dbgen.Queries) error {
		var err error
		rows, err = q.ListAnalysisReadings(ctx)
		if err != nil {
			return fmt.Errorf("load readings: %w", err)
		}
		eventRows, err = q.ListAnalysisEvents(ctx)
		if err != nil {
			return fmt.Errorf("load events: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, 0, err
	}
	readings := make([]analysis.Reading, len(rows))
	meters := 0
	for i, r := range rows {
		if i == 0 || r.MeterID != rows[i-1].MeterID {
			meters++
		}
		readings[i] = analysis.Reading{
			MeterID: r.MeterID, Timestamp: r.ReadingTimestamp,
			ConsumptionKWh: r.ConsumptionKwh, VoltageV: r.VoltageV, CurrentA: r.CurrentA, PowerFactor: r.PowerFactor,
		}
	}
	events := make([]analysis.Event, len(eventRows))
	for i, e := range eventRows {
		events[i] = analysis.Event{
			MeterID: e.MeterID, Timestamp: e.EventTimestamp,
			Type: analysis.EventType(e.EventType), Description: e.Description,
		}
	}
	return readings, events, meters, nil
}

// explain produces one explanation per finding, sequentially: the supplied
// data has a handful of findings, and a local model serves one request at a
// time anyway. The stage has its own budget (Options.ExplanationTimeout);
// when it is exhausted the remaining findings get the deterministic text.
// It returns an error only when the run itself is cancelled.
func (s *Service) explain(ctx context.Context, findings []analysis.Finding, evidence []Evidence, log *slog.Logger) ([]*storedExplanation, error) {
	stageCtx := ctx
	if s.explanationTimeout > 0 {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, s.explanationTimeout)
		defer cancel()
	}
	settings := s.explainers.Settings
	log.InfoContext(ctx, "explanation generation started", "findings", len(findings), "provider", settings.Provider, "model", settings.Model)
	out := make([]*storedExplanation, len(findings))
	for i, f := range findings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out[i] = s.explainOne(ctx, stageCtx, inputOf(f, evidence[i]), log)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// explainOne never fails the run: a primary-provider failure is replaced by
// the fallback text and a sanitized code; if no text can be produced the
// finding is stored without an explanation. Raw provider errors, prompts and
// responses are never logged.
func (s *Service) explainOne(runCtx, stageCtx context.Context, in ExplanationInput, log *slog.Logger) *storedExplanation {
	log = log.With("priority", in.Priority, "meter_id", in.MeterID, "provider", s.explainers.Settings.Provider)
	start := time.Now()
	exp, err := s.explainers.Primary.Explain(stageCtx, in)
	if err == nil {
		log.InfoContext(runCtx, "explanation generated", "source", exp.Source, "model", exp.Model, "duration_ms", time.Since(start).Milliseconds())
		return &storedExplanation{Explanation: exp, GeneratedAt: time.Now().UTC()}
	}
	// A cancelled run is lifecycle control, not provider degradation. Do not
	// call or log the fallback; explain returns the parent cancellation and
	// the worker records the run according to its normal cancellation policy.
	if runCtx.Err() != nil {
		return nil
	}
	code := FallbackCode(err)
	if s.explainers.Fallback != nil {
		if fallback, ferr := s.explainers.Fallback.Explain(runCtx, in); ferr == nil {
			log.WarnContext(runCtx, "explanation fallback used", "fallback_code", code, "duration_ms", time.Since(start).Milliseconds())
			return &storedExplanation{Explanation: fallback, GeneratedAt: time.Now().UTC(), FallbackCode: &code}
		}
	}
	log.WarnContext(runCtx, "explanation unavailable", "fallback_code", code, "duration_ms", time.Since(start).Milliseconds())
	return nil
}

func inputOf(f analysis.Finding, e Evidence) ExplanationInput {
	return ExplanationInput{
		MeterID: f.MeterID, Type: string(f.Type), Severity: string(f.Severity), Confidence: f.Confidence,
		Priority: f.Priority, RecommendedAction: string(f.RecommendedAction), Reason: f.Reason,
		StartedAt: f.StartedAt, LastObservedAt: f.LastObservedAt, Evidence: e,
	}
}

// persist writes the meter statuses and findings (with their explanations)
// and completes the run in one transaction: either all of it becomes
// visible, or none of it does.
func (s *Service) persist(ctx context.Context, id int64, result analysis.Result, evidence []Evidence, explanations []*storedExplanation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.queries.WithTx(tx)

	for _, m := range result.Meters {
		if err := q.InsertAnalysisMeterResult(ctx, dbgen.InsertAnalysisMeterResultParams{
			AnalysisRunID: id, MeterID: m.MeterID, ComputedStatus: string(m.Status),
			EvaluatedReadings: int32(m.EvaluatedReadings), FlaggedReadings: int32(m.FlaggedReadings),
			FindingsCount: int32(m.Findings),
		}); err != nil {
			return fmt.Errorf("insert status of meter %s: %w", m.MeterID, err)
		}
	}
	high, confidenceSum := 0, 0.0
	for i, f := range result.Findings {
		evidenceJSON, err := json.Marshal(evidence[i])
		if err != nil {
			return fmt.Errorf("encode evidence of finding %d: %w", f.Priority, err)
		}
		params := dbgen.InsertAnomalyParams{
			AnalysisRunID:           id,
			MeterID:                 f.MeterID,
			Priority:                int32(f.Priority),
			Type:                    string(f.Type),
			Severity:                string(f.Severity),
			Confidence:              f.Confidence,
			Rule:                    string(f.Rule),
			RecommendedAction:       string(f.RecommendedAction),
			Reason:                  f.Reason,
			StartedAt:               f.StartedAt,
			LastObservedAt:          f.LastObservedAt,
			DurationSeconds:         int64(f.Duration / time.Second),
			ConsumptionDeviationPct: percent(f.Consumption.DeviationPct),
			Evidence:                evidenceJSON,
		}
		if i < len(explanations) && explanations[i] != nil {
			if err := setExplanation(&params, explanations[i]); err != nil {
				return fmt.Errorf("encode explanation of finding %d: %w", f.Priority, err)
			}
		}
		if err := q.InsertAnomaly(ctx, params); err != nil {
			return fmt.Errorf("insert finding %d: %w", f.Priority, err)
		}
		if f.Severity == analysis.SeverityHigh {
			high++
		}
		confidenceSum += f.Confidence
	}
	var aggregate *float64
	if len(result.Findings) > 0 {
		mean := confidenceSum / float64(len(result.Findings))
		aggregate = &mean
	}
	n, err := q.CompleteAnalysisRun(ctx, dbgen.CompleteAnalysisRunParams{
		ID: id, FindingsCount: int32Ptr(len(result.Findings)), HighPriorityCount: int32Ptr(high),
		AggregateConfidence: aggregate,
	})
	if err = affectedOne(n, err); err != nil {
		return fmt.Errorf("complete run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit results: %w", err)
	}
	return nil
}

func setExplanation(p *dbgen.InsertAnomalyParams, e *storedExplanation) error {
	text, err := json.Marshal(e.Text)
	if err != nil {
		return err
	}
	source := string(e.Source)
	fallbackUsed := e.FallbackCode != nil
	promptVersion := e.PromptVersion
	generatedAt := e.GeneratedAt
	p.Explanation = text
	p.ExplanationSource = &source
	p.ExplanationModel = e.Model
	p.ExplanationPromptVersion = &promptVersion
	p.ExplanationGeneratedAt = &generatedAt
	p.ExplanationFallbackUsed = &fallbackUsed
	p.ExplanationFallbackCode = e.FallbackCode
	return nil
}

func int32Ptr(v int) *int32 {
	i := int32(v)
	return &i
}

func intPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}

func runOf(r dbgen.AnalysisRun) Run {
	return Run{
		ID:                  r.ID,
		Status:              Status(r.Status),
		Stage:               Stage(r.Stage),
		Progress:            int(r.ProgressPercent),
		EngineVersion:       r.EngineVersion,
		MetersCount:         intPtr(r.MetersCount),
		ReadingsCount:       intPtr(r.ReadingsCount),
		EventsCount:         intPtr(r.EventsCount),
		FindingsCount:       intPtr(r.FindingsCount),
		HighPriorityCount:   intPtr(r.HighPriorityCount),
		AggregateConfidence: r.AggregateConfidence,
		ErrorCode:           r.ErrorCode,
		ErrorMessage:        r.ErrorMessage,
		CreatedAt:           r.CreatedAt,
		StartedAt:           r.StartedAt,
		CompletedAt:         r.CompletedAt,
	}
}

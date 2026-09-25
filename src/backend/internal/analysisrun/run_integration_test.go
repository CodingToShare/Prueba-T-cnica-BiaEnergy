//go:build integration

package analysisrun

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/ingestion"
	"bia-energy.local/backend/internal/platform/postgres/dbgen"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
	"bia-energy.local/backend/internal/workspace"
)

// analyzerFunc adapts a function to the Analyzer boundary, to inject failures
// without touching the engine.
type analyzerFunc func(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error)

func (f analyzerFunc) Analyze(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error) {
	return f(ctx, r, e)
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func engine(t *testing.T) *analysis.Engine {
	t.Helper()
	e, err := analysis.New(analysis.DefaultConfig())
	require.NoError(t, err)
	return e
}

func newService(t *testing.T, pool *pgxpool.Pool, a Analyzer, opts Options) *Service {
	t.Helper()
	s, err := NewService(pool, a, analysis.EngineVersion, analysis.DefaultConfig(), quiet, opts)
	require.NoError(t, err)
	return s
}

// completeRun queues a run and processes it synchronously.
func completeRun(t *testing.T, s *Service) Run {
	t.Helper()
	ctx := context.Background()
	queued, created, err := s.Request(ctx)
	require.NoError(t, err)
	require.True(t, created)
	processed, err := s.ProcessNext(ctx)
	require.NoError(t, err)
	require.True(t, processed)
	run, err := s.Get(ctx, queued.ID)
	require.NoError(t, err)
	return run
}

// csvResult analyzes the supplied CSV files directly, the Phase 02 path.
func csvResult(t *testing.T) analysis.Result {
	t.Helper()
	dir, err := workspace.FindDir("data/input")
	require.NoError(t, err)
	ds, err := ingestion.ParseDir(dir)
	require.NoError(t, err)
	readings := make([]analysis.Reading, len(ds.Readings))
	for i, r := range ds.Readings {
		readings[i] = analysis.Reading{MeterID: r.MeterID, Timestamp: r.Timestamp, ConsumptionKWh: r.ConsumptionKWh, VoltageV: r.VoltageV, CurrentA: r.CurrentA, PowerFactor: r.PowerFactor}
	}
	events := make([]analysis.Event, len(ds.Events))
	for i, e := range ds.Events {
		events[i] = analysis.Event{MeterID: e.MeterID, Timestamp: e.Timestamp, Type: analysis.EventType(e.Type), Description: e.Description}
	}
	res, err := engine(t).Analyze(context.Background(), readings, events)
	require.NoError(t, err)
	return res
}

func persistedAnomalies(t *testing.T, pool *pgxpool.Pool, runID int64) []dbgen.Anomaly {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT id, meter_id, priority, type, severity, confidence, evidence FROM anomalies WHERE analysis_run_id = $1 ORDER BY priority`, runID)
	require.NoError(t, err)
	defer rows.Close()
	var out []dbgen.Anomaly
	for rows.Next() {
		var a dbgen.Anomaly
		require.NoError(t, rows.Scan(&a.ID, &a.MeterID, &a.Priority, &a.Type, &a.Severity, &a.Confidence, &a.Evidence))
		out = append(out, a)
	}
	require.NoError(t, rows.Err())
	return out
}

func countWhere(t *testing.T, pool *pgxpool.Pool, table string, runID int64) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE analysis_run_id = $1", runID).Scan(&n))
	return n
}

func TestRun_SuppliedDatasetFromPostgreSQL_CompletesWithPrioritizedFindings(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newService(t, db.Pool, engine(t), Options{})

	run := completeRun(t, s)

	assert.Equal(t, StatusCompleted, run.Status)
	assert.Equal(t, StageCompleted, run.Stage)
	assert.Equal(t, 100, run.Progress)
	assert.Equal(t, analysis.EngineVersion, run.EngineVersion)
	require.NotNil(t, run.StartedAt)
	require.NotNil(t, run.CompletedAt)
	assert.False(t, run.CompletedAt.Before(*run.StartedAt))
	assert.Nil(t, run.ErrorCode)
	assert.Equal(t, 12, *run.MetersCount)
	assert.Equal(t, 4032, *run.ReadingsCount)
	assert.Equal(t, 4, *run.EventsCount)
	assert.Equal(t, 4, *run.FindingsCount)
	assert.Equal(t, 2, *run.HighPriorityCount, "high priority = severity HIGH")

	// The database path gives exactly the result of analyzing the CSV files.
	want := csvResult(t)
	got := persistedAnomalies(t, db.Pool, run.ID)
	require.Len(t, got, len(want.Findings))
	var order []string
	sum := 0.0
	for i, f := range want.Findings {
		order = append(order, got[i].MeterID)
		assert.Equal(t, f.MeterID, got[i].MeterID)
		assert.Equal(t, int32(f.Priority), got[i].Priority)
		assert.Equal(t, string(f.Type), got[i].Type)
		assert.Equal(t, string(f.Severity), got[i].Severity)
		assert.Equal(t, f.Confidence, got[i].Confidence, "confidence is stored without precision loss")
		sum += f.Confidence
	}
	assert.Equal(t, []string{"M-109", "M-112", "M-104", "M-106"}, order)
	require.NotNil(t, run.AggregateConfidence)
	assert.InDelta(t, sum/4, *run.AggregateConfidence, 1e-15)

	// The configuration snapshot is the engine's actual configuration.
	var raw []byte
	require.NoError(t, db.Pool.QueryRow(context.Background(), "SELECT configuration FROM analysis_runs WHERE id = $1", run.ID).Scan(&raw))
	var snapshot analysis.Config
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	assert.Equal(t, analysis.DefaultConfig(), snapshot)

	// Meter statuses are the engine's, stored per run.
	statuses := map[string]string{}
	rows, err := db.Pool.Query(context.Background(), "SELECT meter_id, computed_status FROM analysis_meter_results WHERE analysis_run_id = $1", run.ID)
	require.NoError(t, err)
	for rows.Next() {
		var m, s string
		require.NoError(t, rows.Scan(&m, &s))
		statuses[m] = s
	}
	require.NoError(t, rows.Err())
	assert.Len(t, statuses, 12)
	assert.Equal(t, "CRITICAL", statuses["M-109"])
	assert.Equal(t, "ALERT", statuses["M-112"])
	assert.Equal(t, "ALERT", statuses["M-104"])
	assert.Equal(t, "OK", statuses["M-106"])
	assert.Equal(t, "OK", statuses["M-101"])
}

func TestRun_EvidenceRoundTripsThroughJSONB(t *testing.T) {
	db := pgtest.StartSeeded(t)
	run := completeRun(t, newService(t, db.Pool, engine(t), Options{}))
	want := csvResult(t)

	byMeter := map[string]Evidence{}
	for i, a := range persistedAnomalies(t, db.Pool, run.ID) {
		var got Evidence
		require.NoError(t, json.Unmarshal(a.Evidence, &got))
		assert.Equal(t, EvidenceOf(want.Findings[i]), got, "%s: evidence is identical after the JSONB round trip", a.MeterID)
		byMeter[a.MeterID] = got
	}

	m109 := byMeter["M-109"]
	assert.Greater(t, m109.Consumption.MedianDeviationPct, 90.0)
	assert.Greater(t, m109.Consumption.ObservedKWh, m109.Consumption.BaselineKWh*1.9)
	assert.True(t, metricNamed(m109, "current_a").Corroborates)
	assert.Equal(t, "UP", *metricNamed(m109, "current_a").Direction)
	assert.True(t, metricNamed(m109, "power_factor").Corroborates)
	assert.Equal(t, "DOWN", *metricNamed(m109, "power_factor").Direction)
	assert.True(t, m109.Persistence.Sustained)
	assert.Equal(t, 58.0, m109.Persistence.DurationHours)
	assert.False(t, m109.Persistence.Recovery.Recovered)
	require.Len(t, m109.RelatedEvents, 1)
	assert.Equal(t, "UNKNOWN", m109.RelatedEvents[0].Type)
	assert.Equal(t, "CONTEXT", m109.RelatedEvents[0].Role)
	assert.Equal(t, "No operational event reported", m109.RelatedEvents[0].Description)
	assert.Positive(t, m109.ConfidenceBreakdown.SignalStrength)
	assert.NotEmpty(t, m109.Signals)

	m112 := byMeter["M-112"]
	assert.Zero(t, metricNamed(m112, "consumption_kwh").TriggeredReadings, "consumption stays on its baseline")
	assert.Less(t, m112.Consumption.DeviationPct, 5.0)
	assert.True(t, metricNamed(m112, "voltage_v").Corroborates)
	assert.True(t, metricNamed(m112, "current_a").Corroborates)
	require.Len(t, m112.RelatedEvents, 1)
	assert.Equal(t, "DATA_QUALITY", m112.RelatedEvents[0].Type)
	assert.Equal(t, "CORROBORATES", m112.RelatedEvents[0].Role)

	var action string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		"SELECT recommended_action FROM anomalies WHERE analysis_run_id = $1 AND meter_id = 'M-109'", run.ID).Scan(&action))
	assert.Equal(t, "INVESTIGATE_METER_AND_INSTALLATION", action)
}

func metricNamed(e Evidence, name string) MetricEvidence {
	for _, m := range e.Metrics {
		if m.Metric == name {
			return m
		}
	}
	return MetricEvidence{}
}

func TestRun_AnalyzerFailure_FailsSafelyAndKeepsTheCompletedRunCurrent(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	a := completeRun(t, newService(t, db.Pool, engine(t), Options{}))

	failing := newService(t, db.Pool, analyzerFunc(func(context.Context, []analysis.Reading, []analysis.Event) (analysis.Result, error) {
		return analysis.Result{}, errors.New("internal detail that must not leak")
	}), Options{})
	b, created, err := failing.Request(ctx)
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, StatusQueued, b.Status)
	assert.Equal(t, 0, b.Progress)
	processed, err := failing.ProcessNext(ctx)
	require.NoError(t, err)
	require.True(t, processed)

	b, err = failing.Get(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, b.Status)
	assert.Equal(t, StageFailed, b.Stage)
	assert.Equal(t, Progress(StageAnalyzing), b.Progress, "progress stays where it failed")
	assert.Equal(t, CodeAnalysisFailed, *b.ErrorCode)
	assert.Equal(t, FailureMessage(CodeAnalysisFailed), *b.ErrorMessage)
	assert.NotContains(t, *b.ErrorMessage, "internal detail")
	require.NotNil(t, b.CompletedAt, "a failed run is terminal")
	assert.Nil(t, b.FindingsCount)
	assert.Zero(t, countWhere(t, db.Pool, "anomalies", b.ID))
	assert.Zero(t, countWhere(t, db.Pool, "analysis_meter_results", b.ID))

	current, err := LatestCompleted(ctx, dbgen.New(db.Pool))
	require.NoError(t, err)
	assert.Equal(t, a.ID, current.ID, "a failed run never replaces the current state")
}

func TestRun_PersistenceFailure_RollsBackEveryRow(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	real := engine(t)
	// A finding for a meter that does not exist violates the foreign key
	// after four findings and twelve meter statuses were already inserted.
	broken := newService(t, db.Pool, analyzerFunc(func(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error) {
		res, err := real.Analyze(ctx, r, e)
		if err != nil {
			return res, err
		}
		ghost := res.Findings[0]
		ghost.MeterID, ghost.Priority = "NO-SUCH-METER", len(res.Findings)+1
		res.Findings = append(res.Findings, ghost)
		return res, nil
	}), Options{})

	queued, _, err := broken.Request(ctx)
	require.NoError(t, err)
	_, err = broken.ProcessNext(ctx)
	require.NoError(t, err)

	run, err := broken.Get(ctx, queued.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, run.Status)
	assert.Equal(t, CodePersistenceFailed, *run.ErrorCode)
	assert.Equal(t, Progress(StagePersistingResults), run.Progress)
	assert.Zero(t, countWhere(t, db.Pool, "anomalies", run.ID), "no partial findings")
	assert.Zero(t, countWhere(t, db.Pool, "analysis_meter_results", run.ID), "no partial meter statuses")
	current, err := LatestCompleted(ctx, dbgen.New(db.Pool))
	require.NoError(t, err)
	assert.Nil(t, current, "nothing became current")
}

func TestRequest_ConcurrentRequests_ShareOneActiveRun(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newService(t, db.Pool, engine(t), Options{})
	const callers = 12
	var wg sync.WaitGroup
	start := make(chan struct{})
	ids := make([]int64, callers)
	created := make([]bool, callers)
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var run Run
			run, created[i], errs[i] = s.Request(context.Background())
			ids[i] = run.ID
		}()
	}
	close(start)
	wg.Wait()

	createdCount := 0
	for i := range callers {
		require.NoError(t, errs[i])
		assert.Equal(t, ids[0], ids[i], "every caller gets the same active run")
		if created[i] {
			createdCount++
		}
	}
	assert.Equal(t, 1, createdCount)
	var runs int
	require.NoError(t, db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM analysis_runs").Scan(&runs))
	assert.Equal(t, 1, runs)
}

func TestClaim_CompetingWorkers_NeverClaimTheSameRun(t *testing.T) {
	db := pgtest.StartMigrated(t)
	ctx := context.Background()
	q := dbgen.New(db.Pool)
	for round := range 20 {
		run, err := q.CreateAnalysisRun(ctx, dbgen.CreateAnalysisRunParams{EngineVersion: "test", Configuration: []byte(`{}`)})
		require.NoError(t, err)

		var wg sync.WaitGroup
		start := make(chan struct{})
		claimed := make([]bool, 2)
		errs := make([]error, 2)
		for w := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := q.ClaimQueuedAnalysisRun(ctx, dbgen.ClaimQueuedAnalysisRunParams{ProgressPercent: 10, EngineVersion: "test", Configuration: []byte(`{}`)})
				claimed[w] = err == nil
				errs[w] = err
			}()
		}
		close(start)
		wg.Wait()
		assert.NotEqual(t, claimed[0], claimed[1], "round %d: exactly one worker claims the run", round)
		for i, err := range errs {
			if !claimed[i] {
				require.ErrorIs(t, err, pgx.ErrNoRows)
			}
		}

		code, msg := "test", "test"
		_, err = q.FailAnalysisRun(ctx, dbgen.FailAnalysisRunParams{ID: run.ID, ErrorCode: &code, ErrorMessage: &msg})
		require.NoError(t, err)
	}
}

func TestRecoverInterrupted_FailsRunningRunsAndKeepsQueuedRuns(t *testing.T) {
	ctx := context.Background()

	t.Run("RUNNING becomes FAILED interrupted", func(t *testing.T) {
		db := pgtest.StartSeeded(t)
		s := newService(t, db.Pool, engine(t), Options{})
		queued, _, err := s.Request(ctx)
		require.NoError(t, err)
		_, err = dbgen.New(db.Pool).ClaimQueuedAnalysisRun(ctx, dbgen.ClaimQueuedAnalysisRunParams{ProgressPercent: 10, EngineVersion: "test", Configuration: []byte(`{}`)}) // a crashed process left it RUNNING
		require.NoError(t, err)

		recovered, err := s.RecoverInterrupted(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int64{queued.ID}, recovered)
		run, err := s.Get(ctx, queued.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusFailed, run.Status)
		assert.Equal(t, CodeInterrupted, *run.ErrorCode)

		next := completeRun(t, s)
		assert.Equal(t, StatusCompleted, next.Status, "a new run can start after recovery")
	})

	t.Run("QUEUED stays processable", func(t *testing.T) {
		db := pgtest.StartSeeded(t)
		s := newService(t, db.Pool, engine(t), Options{})
		queued, _, err := s.Request(ctx)
		require.NoError(t, err)

		recovered, err := s.RecoverInterrupted(ctx)
		require.NoError(t, err)
		assert.Empty(t, recovered)
		processed, err := s.ProcessNext(ctx)
		require.NoError(t, err)
		require.True(t, processed)
		run, err := s.Get(ctx, queued.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusCompleted, run.Status)
	})
}

// blockingAnalyzer waits until its context ends, signalling when it started.
func blockingAnalyzer(started chan<- struct{}) Analyzer {
	return analyzerFunc(func(ctx context.Context, _ []analysis.Reading, _ []analysis.Event) (analysis.Result, error) {
		close(started)
		<-ctx.Done()
		return analysis.Result{}, ctx.Err()
	})
}

func TestRun_Timeout_FailsWithTimeoutCode(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	s := newService(t, db.Pool, blockingAnalyzer(make(chan struct{})), Options{RunTimeout: 200 * time.Millisecond})

	queued, _, err := s.Request(ctx)
	require.NoError(t, err)
	_, err = s.ProcessNext(ctx)
	require.NoError(t, err)

	run, err := s.Get(ctx, queued.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, run.Status)
	assert.Equal(t, CodeTimeout, *run.ErrorCode)
}

func TestWork_ShutdownInterruptsTheRunningAnalysisAndStops(t *testing.T) {
	db := pgtest.StartSeeded(t)
	started := make(chan struct{})
	s := newService(t, db.Pool, blockingAnalyzer(started), Options{})
	queued, _, err := s.Request(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Work(ctx); close(done) }()
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("the worker did not start the queued run")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the worker did not stop after cancellation")
	}

	run, err := s.Get(context.Background(), queued.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, run.Status, "an interrupted run is not left RUNNING")
	assert.Equal(t, CodeInterrupted, *run.ErrorCode)
}

func TestWork_WakeUpProcessesARequestedRun(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newService(t, db.Pool, engine(t), Options{PollInterval: time.Hour}) // only the wake-up can trigger it
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Work(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	queued, _, err := s.Request(context.Background())
	require.NoError(t, err)
	deadline := time.Now().Add(30 * time.Second)
	for {
		run, err := s.Get(context.Background(), queued.ID)
		require.NoError(t, err)
		if run.Status == StatusCompleted {
			break
		}
		require.NotEqual(t, StatusFailed, run.Status)
		require.True(t, time.Now().Before(deadline), "run did not complete in time")
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLatestCompleted_RunningOrFailedRunsNeverReplaceIt(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	q := dbgen.New(db.Pool)
	current := func() int64 {
		c, err := LatestCompleted(ctx, q)
		require.NoError(t, err)
		require.NotNil(t, c)
		return c.ID
	}

	a := completeRun(t, newService(t, db.Pool, engine(t), Options{}))
	assert.Equal(t, a.ID, current())

	failing := newService(t, db.Pool, analyzerFunc(func(context.Context, []analysis.Reading, []analysis.Event) (analysis.Result, error) {
		return analysis.Result{}, errors.New("boom")
	}), Options{})
	b, _, err := failing.Request(ctx)
	require.NoError(t, err)
	assert.Equal(t, a.ID, current(), "while B is QUEUED")
	_, err = q.ClaimQueuedAnalysisRun(ctx, dbgen.ClaimQueuedAnalysisRunParams{ProgressPercent: 10, EngineVersion: "test", Configuration: []byte(`{}`)})
	require.NoError(t, err)
	assert.Equal(t, a.ID, current(), "while B is RUNNING")
	code, msg := CodeAnalysisFailed, FailureMessage(CodeAnalysisFailed)
	_, err = q.FailAnalysisRun(ctx, dbgen.FailAnalysisRunParams{ID: b.ID, ErrorCode: &code, ErrorMessage: &msg})
	require.NoError(t, err)
	assert.Equal(t, a.ID, current(), "after B FAILED")

	c := completeRun(t, newService(t, db.Pool, engine(t), Options{}))
	assert.Equal(t, c.ID, current(), "C becomes current once COMPLETED")
	assert.Equal(t, 4, countWhere(t, db.Pool, "anomalies", a.ID), "history is kept")
}

func TestRun_NoLongerRunning_WritesNoResults(t *testing.T) {
	// Another actor (e.g. recovery) fails the run while the engine works: the
	// worker must not write findings for it, and the first failure stays.
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	real := engine(t)
	var runID int64
	s := newService(t, db.Pool, analyzerFunc(func(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error) {
		code, msg := CodeInterrupted, FailureMessage(CodeInterrupted)
		_, err := dbgen.New(db.Pool).FailAnalysisRun(ctx, dbgen.FailAnalysisRunParams{ID: runID, ErrorCode: &code, ErrorMessage: &msg})
		require.NoError(t, err)
		return real.Analyze(ctx, r, e)
	}), Options{})
	queued, _, err := s.Request(ctx)
	require.NoError(t, err)
	runID = queued.ID
	_, err = s.ProcessNext(ctx)
	require.NoError(t, err)

	run, err := s.Get(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, run.Status)
	assert.Equal(t, CodeInterrupted, *run.ErrorCode, "the first recorded failure is kept")
	assert.Zero(t, countWhere(t, db.Pool, "anomalies", runID))
	assert.Zero(t, countWhere(t, db.Pool, "analysis_meter_results", runID))
}

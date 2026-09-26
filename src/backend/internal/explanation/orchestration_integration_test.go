//go:build integration

package explanation_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/anomaly"
	"bia-energy.local/backend/internal/explanation"
	"bia-energy.local/backend/internal/platform/metrics"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

// Explanation generation inside real analysis runs, on the supplied dataset
// in PostgreSQL 18.6. The Ollama provider talks to a controlled local fake;
// no model is needed.

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeOllama struct {
	srv      *httptest.Server
	requests atomic.Int64
}

func newFakeOllama(t *testing.T, handler http.HandlerFunc) *fakeOllama {
	t.Helper()
	f := &fakeOllama{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		handler(w, r)
	}))
	t.Cleanup(func() {
		f.srv.CloseClientConnections()
		f.srv.Close()
	})
	return f
}

func groundedReply(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var payload struct {
		ClassificationMeaning string `json:"classification_meaning"`
		EvidenceNarrative     string `json:"evidence_narrative"`
		RecommendedAction     struct {
			Description string `json:"description"`
		} `json:"recommended_action"`
	}
	if err := json.Unmarshal([]byte(request.Messages[1].Content), &payload); err != nil {
		http.Error(w, "bad evidence", http.StatusBadRequest)
		return
	}
	content, _ := json.Marshal(map[string]string{
		"summary":                 "The meter's consumption departed from its hourly baseline.",
		"why_it_matters":          payload.ClassificationMeaning,
		"evidence_narrative":      payload.EvidenceNarrative,
		"recommended_action_text": payload.RecommendedAction.Description,
	})
	reply(string(content))(w, r)
}

func reply(content string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		b, _ := json.Marshal(map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "done": true})
		_, _ = w.Write(b)
	}
}

func deterministicExplainers() analysisrun.Explainers {
	return analysisrun.Explainers{Primary: explanation.Deterministic{}, Settings: explanation.DeterministicSettings()}
}

func ollamaExplainers(t *testing.T, baseURL string, timeout time.Duration) analysisrun.Explainers {
	t.Helper()
	o, err := explanation.NewOllama(explanation.OllamaConfig{BaseURL: baseURL, Model: "fake-model:1b", Timeout: timeout})
	require.NoError(t, err)
	return analysisrun.Explainers{Primary: o, Fallback: explanation.Deterministic{}, Settings: o.Settings()}
}

func newEngine(t *testing.T) *analysis.Engine {
	t.Helper()
	e, err := analysis.New(analysis.DefaultConfig())
	require.NoError(t, err)
	return e
}

func runOnce(t *testing.T, pool *pgxpool.Pool, analyzer analysisrun.Analyzer, ex analysisrun.Explainers, opts analysisrun.Options) analysisrun.Run {
	t.Helper()
	s, err := analysisrun.NewService(pool, analyzer, ex, analysis.EngineVersion, analysis.DefaultConfig(), quiet, opts)
	require.NoError(t, err)
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

// stored is one persisted finding with its explanation provenance.
type stored struct {
	ID           int64
	Priority     int
	MeterID      string
	Type         string
	Severity     string
	Confidence   float64
	Action       string
	Evidence     string
	Source       *string
	Model        *string
	Prompt       *string
	FallbackUsed *bool
	FallbackCode *string
}

func findings(t *testing.T, pool *pgxpool.Pool, runID int64) []stored {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id, priority, meter_id, type, severity, confidence, recommended_action, evidence::text,
		       explanation_source, explanation_model, explanation_prompt_version,
		       explanation_fallback_used, explanation_fallback_code
		FROM anomalies WHERE analysis_run_id = $1 ORDER BY priority`, runID)
	require.NoError(t, err)
	defer rows.Close()
	var out []stored
	for rows.Next() {
		var s stored
		require.NoError(t, rows.Scan(&s.ID, &s.Priority, &s.MeterID, &s.Type, &s.Severity, &s.Confidence, &s.Action, &s.Evidence,
			&s.Source, &s.Model, &s.Prompt, &s.FallbackUsed, &s.FallbackCode))
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	return out
}

type outcome struct {
	MeterID, Type, Severity, Action string
	Confidence                      float64
	Priority                        int
}

func outcomes(fs []stored) []outcome {
	out := make([]outcome, len(fs))
	for i, f := range fs {
		out[i] = outcome{f.MeterID, f.Type, f.Severity, f.Action, f.Confidence, f.Priority}
	}
	return out
}

func requireCompleted(t *testing.T, run analysisrun.Run) {
	t.Helper()
	require.Equal(t, analysisrun.StatusCompleted, run.Status, "error: %v", run.ErrorCode)
	require.NotNil(t, run.FindingsCount)
	require.Equal(t, 4, *run.FindingsCount, "the supplied data yields four findings")
}

// A. Deterministic configuration → deterministic explanations, read back
// through the anomaly service exactly as the API serves them.
func TestRun_DeterministicProvider_PersistsAnExplanationForEveryFinding(t *testing.T) {
	db := pgtest.StartSeeded(t)
	before := time.Now().UTC().Add(-time.Second)
	run := runOnce(t, db.Pool, newEngine(t), deterministicExplainers(), analysisrun.Options{})
	requireCompleted(t, run)

	var settings string
	require.NoError(t, db.Pool.QueryRow(context.Background(), "SELECT explanation_configuration::text FROM analysis_runs WHERE id = $1", run.ID).Scan(&settings))
	assert.JSONEq(t, `{"provider":"deterministic","model":null,"prompt_version":"evidence-template-v1"}`, settings)

	svc := anomaly.NewService(db.Pool)
	for _, f := range findings(t, db.Pool, run.ID) {
		assert.Equal(t, "DETERMINISTIC", *f.Source)
		assert.False(t, *f.FallbackUsed)
		assert.Nil(t, f.FallbackCode)
		assert.Nil(t, f.Model)

		d, err := svc.Get(context.Background(), f.ID)
		require.NoError(t, err)
		require.NotNil(t, d.Explanation)
		e := d.Explanation
		assert.Equal(t, "DETERMINISTIC", e.Source)
		assert.Equal(t, explanation.TemplateVersion, e.PromptVersion)
		assert.False(t, e.FallbackUsed)
		assert.Nil(t, e.Model)
		assert.WithinDuration(t, time.Now().UTC(), e.GeneratedAt, time.Minute)
		assert.True(t, e.GeneratedAt.After(before))
		for _, text := range []string{e.Summary, e.WhyItMatters, e.EvidenceNarrative, e.RecommendedActionText} {
			assert.NotEmpty(t, text)
		}
	}
}

// B. Ollama succeeds → generated text persisted with its model and prompt
// version; one request per finding, sequentially.
func TestRun_OllamaSucceeds_PersistsGeneratedTextWithProvenance(t *testing.T) {
	db := pgtest.StartSeeded(t)
	fake := newFakeOllama(t, groundedReply)
	run := runOnce(t, db.Pool, newEngine(t), ollamaExplainers(t, fake.srv.URL, 5*time.Second), analysisrun.Options{ExplanationTimeout: time.Minute})
	requireCompleted(t, run)

	assert.EqualValues(t, 4, fake.requests.Load())
	var settings string
	require.NoError(t, db.Pool.QueryRow(context.Background(), "SELECT explanation_configuration::text FROM analysis_runs WHERE id = $1", run.ID).Scan(&settings))
	assert.JSONEq(t, `{"provider":"ollama","model":"fake-model:1b","prompt_version":"energy-explanation-v1","timeout_ms":5000}`, settings)
	assert.NotContains(t, settings, "127.0.0.1", "the provider URL is not recorded")

	svc := anomaly.NewService(db.Pool)
	for _, f := range findings(t, db.Pool, run.ID) {
		d, err := svc.Get(context.Background(), f.ID)
		require.NoError(t, err)
		require.NotNil(t, d.Explanation)
		assert.Equal(t, "OLLAMA", d.Explanation.Source)
		assert.Equal(t, "fake-model:1b", *d.Explanation.Model)
		assert.Equal(t, explanation.PromptVersion, d.Explanation.PromptVersion)
		assert.False(t, d.Explanation.FallbackUsed)
		assert.Equal(t, "The meter's consumption departed from its hourly baseline.", d.Explanation.Summary)
	}
	assert.EqualValues(t, 4, fake.requests.Load(), "reading persisted explanations never calls the provider")
}

func TestRun_PartialProviderFailures_PersistIndependentFindingProvenance(t *testing.T) {
	db := pgtest.StartSeeded(t)
	var requestNumber atomic.Int64
	release := make(chan struct{})
	defer close(release)
	fake := newFakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
		switch requestNumber.Add(1) {
		case 1, 3:
			groundedReply(w, r)
		case 2:
			http.Error(w, "controlled failure", http.StatusInternalServerError)
		default:
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}
	})
	run := runOnce(t, db.Pool, newEngine(t), ollamaExplainers(t, fake.srv.URL, 150*time.Millisecond), analysisrun.Options{ExplanationTimeout: time.Minute})
	requireCompleted(t, run)

	fs := findings(t, db.Pool, run.ID)
	require.Len(t, fs, 4)
	assert.Equal(t, "OLLAMA", *fs[0].Source)
	assert.Equal(t, "DETERMINISTIC", *fs[1].Source)
	assert.Equal(t, analysisrun.FallbackProviderError, *fs[1].FallbackCode)
	assert.Equal(t, "OLLAMA", *fs[2].Source)
	assert.Equal(t, "DETERMINISTIC", *fs[3].Source)
	assert.Equal(t, analysisrun.FallbackTimeout, *fs[3].FallbackCode)

	svc := anomaly.NewService(db.Pool)
	for i, f := range fs {
		detail, err := svc.Get(context.Background(), f.ID)
		require.NoError(t, err)
		require.NotNil(t, detail.Explanation)
		assert.Equal(t, *f.Source, detail.Explanation.Source, "finding %d", i+1)
		assert.Equal(t, *f.FallbackUsed, detail.Explanation.FallbackUsed, "finding %d", i+1)
	}
}

// C–E. Ollama unreachable, malformed or too slow → deterministic fallback
// with a sanitized code; the analytical run still COMPLETES and its results
// are identical to a deterministic run.
func TestRun_OllamaFailures_FallBackAndTheRunStillCompletes(t *testing.T) {
	db := pgtest.StartSeeded(t)
	reference := outcomes(findings(t, db.Pool, runOnce(t, db.Pool, newEngine(t), deterministicExplainers(), analysisrun.Options{}).ID))

	closed := httptest.NewServer(http.NotFoundHandler())
	unreachable := closed.URL
	closed.Close()
	release := make(chan struct{})
	defer close(release)
	hanging := newFakeOllama(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})

	cases := []struct {
		name    string
		baseURL string
		timeout time.Duration
		code    string
	}{
		{"unreachable", unreachable, 5 * time.Second, analysisrun.FallbackProviderUnavailable},
		{"malformed JSON", newFakeOllama(t, reply("not json at all")).srv.URL, 5 * time.Second, analysisrun.FallbackInvalidResponse},
		{"HTTP 500", newFakeOllama(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }).srv.URL, 5 * time.Second, analysisrun.FallbackProviderError},
		{"invented number", newFakeOllama(t, reply(`{"summary":"Consumption rose 777% above normal.","why_it_matters":"a","evidence_narrative":"b","recommended_action_text":"c"}`)).srv.URL, 5 * time.Second, analysisrun.FallbackValidationFailed},
		{"timeout", hanging.srv.URL, 150 * time.Millisecond, analysisrun.FallbackTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runOnce(t, db.Pool, newEngine(t), ollamaExplainers(t, tc.baseURL, tc.timeout), analysisrun.Options{ExplanationTimeout: time.Minute})
			requireCompleted(t, run)

			fs := findings(t, db.Pool, run.ID)
			assert.Equal(t, reference, outcomes(fs), "type, severity, confidence, priority and action are unchanged")
			for _, f := range fs {
				assert.Equal(t, "DETERMINISTIC", *f.Source)
				assert.True(t, *f.FallbackUsed)
				assert.Equal(t, tc.code, *f.FallbackCode)
				assert.Nil(t, f.Model, "fallback text is deterministic: no model is attributed")
				assert.Equal(t, explanation.TemplateVersion, *f.Prompt)
			}
		})
	}
}

// The stage budget bounds a slow model across all findings: once exhausted,
// the remaining findings get the deterministic text immediately.
func TestRun_ExplanationStageBudget_DegradesToTheFallbackWithoutFailingTheRun(t *testing.T) {
	db := pgtest.StartSeeded(t)
	release := make(chan struct{})
	defer close(release)
	slow := newFakeOllama(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})

	start := time.Now()
	run := runOnce(t, db.Pool, newEngine(t), ollamaExplainers(t, slow.srv.URL, time.Minute), analysisrun.Options{ExplanationTimeout: 300 * time.Millisecond})
	requireCompleted(t, run)
	assert.Less(t, time.Since(start), 30*time.Second, "one slow call cannot hold the run for the per-call timeout")
	for _, f := range findings(t, db.Pool, run.ID) {
		assert.True(t, *f.FallbackUsed)
		assert.Equal(t, analysisrun.FallbackTimeout, *f.FallbackCode)
	}
}

// F. An analytics failure fails the run even with a generative provider
// configured: explanation fallback never masks an analytical failure.
func TestRun_AnalyticsFailure_FailsTheRunAndNoExplanationIsRequested(t *testing.T) {
	db := pgtest.StartSeeded(t)
	fake := newFakeOllama(t, groundedReply)
	failing := analyzerFunc(func(context.Context, []analysis.Reading, []analysis.Event) (analysis.Result, error) {
		return analysis.Result{}, errors.New("engine exploded")
	})

	run := runOnce(t, db.Pool, failing, ollamaExplainers(t, fake.srv.URL, time.Second), analysisrun.Options{ExplanationTimeout: time.Minute})

	assert.Equal(t, analysisrun.StatusFailed, run.Status)
	require.NotNil(t, run.ErrorCode)
	assert.Equal(t, analysisrun.CodeAnalysisFailed, *run.ErrorCode)
	assert.EqualValues(t, 0, fake.requests.Load())
	assert.Equal(t, 0, pgtest.Count(t, db.Pool, "anomalies"))
}

// When no text can be produced at all, the finding is still stored, without
// an explanation, and the API reports explanation = null.
func TestRun_NoExplanationAvailable_StoresTheFindingWithoutOne(t *testing.T) {
	db := pgtest.StartSeeded(t)
	broken := analysisrun.Explainers{Primary: providerFunc(func(context.Context, analysisrun.ExplanationInput) (analysisrun.Explanation, error) {
		return analysisrun.Explanation{}, errors.New("nothing available")
	}), Settings: analysisrun.ExplanationSettings{Provider: "broken", PromptVersion: "none"}}

	run := runOnce(t, db.Pool, newEngine(t), broken, analysisrun.Options{})
	requireCompleted(t, run)
	fs := findings(t, db.Pool, run.ID)
	for _, f := range fs {
		assert.Nil(t, f.Source)
		d, err := anomaly.NewService(db.Pool).Get(context.Background(), f.ID)
		require.NoError(t, err)
		assert.Nil(t, d.Explanation)
	}
}

func TestSchema_RejectsInconsistentExplanationProvenance(t *testing.T) {
	db := pgtest.StartSeeded(t)
	run := runOnce(t, db.Pool, newEngine(t), deterministicExplainers(), analysisrun.Options{})
	id := findings(t, db.Pool, run.ID)[0].ID
	ctx := context.Background()

	for name, update := range map[string]string{
		"text without source":            `UPDATE anomalies SET explanation_source = NULL WHERE id = $1`,
		"unknown source":                 `UPDATE anomalies SET explanation_source = 'GPT' WHERE id = $1`,
		"model on deterministic text":    `UPDATE anomalies SET explanation_model = 'm' WHERE id = $1`,
		"fallback without code":          `UPDATE anomalies SET explanation_fallback_used = true WHERE id = $1`,
		"generated text marked fallback": `UPDATE anomalies SET explanation_source = 'OLLAMA', explanation_fallback_used = true, explanation_fallback_code = 'timeout' WHERE id = $1`,
		"explanation not an object":      `UPDATE anomalies SET explanation = '"text"' WHERE id = $1`,
	} {
		_, err := db.Pool.Exec(ctx, update, id)
		assert.Error(t, err, name)
	}
}

type analyzerFunc func(context.Context, []analysis.Reading, []analysis.Event) (analysis.Result, error)

func (f analyzerFunc) Analyze(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error) {
	return f(ctx, r, e)
}

type providerFunc func(context.Context, analysisrun.ExplanationInput) (analysisrun.Explanation, error)

func (f providerFunc) Explain(ctx context.Context, in analysisrun.ExplanationInput) (analysisrun.Explanation, error) {
	return f(ctx, in)
}

// Phase 06: run and explanation metrics reflect real lifecycle events. A
// completed run is counted after its transaction, every explanation once,
// fallbacks by sanitized code, and failed runs separately; no identifier is
// ever a label.
func TestRun_Metrics_CountRunsExplanationsAndFallbacks(t *testing.T) {
	db := pgtest.StartSeeded(t)
	m := metrics.New()
	value := func(name string, labels map[string]string) float64 {
		t.Helper()
		v, _ := metrics.Value(m.Gatherer(), name, labels)
		return v
	}

	requireCompleted(t, runOnce(t, db.Pool, newEngine(t), deterministicExplainers(), analysisrun.Options{Metrics: m}))
	assert.Equal(t, 1.0, value("bia_analysis_runs_total", map[string]string{"status": "completed"}))
	assert.Equal(t, 1.0, value("bia_analysis_findings", nil))
	assert.Equal(t, 4.0, value("bia_explanation_generation_total", map[string]string{"provider": "deterministic", "outcome": "generated"}))
	assert.Equal(t, 0.0, value("bia_analysis_runs_active", nil))

	closed := httptest.NewServer(http.NotFoundHandler())
	unreachable := closed.URL
	closed.Close()
	requireCompleted(t, runOnce(t, db.Pool, newEngine(t), ollamaExplainers(t, unreachable, 5*time.Second), analysisrun.Options{Metrics: m, ExplanationTimeout: time.Minute}))
	assert.Equal(t, 2.0, value("bia_analysis_runs_total", map[string]string{"status": "completed"}))
	assert.Equal(t, 4.0, value("bia_explanation_generation_total", map[string]string{"provider": "ollama", "outcome": "fallback"}))
	assert.Equal(t, 4.0, value("bia_explanation_fallback_total", map[string]string{"fallback_code": analysisrun.FallbackProviderUnavailable}))
	assert.Equal(t, 0.0, value("bia_explanation_generation_total", map[string]string{"provider": "ollama", "outcome": "generated"}))

	failing := analyzerFunc(func(context.Context, []analysis.Reading, []analysis.Event) (analysis.Result, error) {
		return analysis.Result{}, errors.New("engine exploded")
	})
	run := runOnce(t, db.Pool, failing, deterministicExplainers(), analysisrun.Options{Metrics: m})
	assert.Equal(t, analysisrun.StatusFailed, run.Status)
	assert.Equal(t, 1.0, value("bia_analysis_runs_total", map[string]string{"status": "failed"}))
	assert.Equal(t, 2.0, value("bia_analysis_runs_total", map[string]string{"status": "completed"}), "a failed run is not counted as completed")
	assert.Equal(t, 4.0, value("bia_explanation_generation_total", map[string]string{"provider": "deterministic", "outcome": "generated"}), "no explanation is requested for a failed analysis")
	assert.Equal(t, 0.0, value("bia_analysis_runs_active", nil))

	families, err := m.Gatherer().Gather()
	require.NoError(t, err)
	for _, mf := range families {
		for _, sample := range mf.GetMetric() {
			for _, label := range sample.GetLabel() {
				assert.NotRegexp(t, `M-1\d\d|^\d+$`, label.GetValue(), "%s{%s}", mf.GetName(), label.GetName())
				assert.NotContains(t, []string{"meter_id", "analysis_id", "anomaly_id", "request_id", "model"}, label.GetName())
			}
		}
	}
}

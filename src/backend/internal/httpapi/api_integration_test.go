//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/anomaly"
	"bia-energy.local/backend/internal/auth"
	"bia-energy.local/backend/internal/dashboard"
	"bia-energy.local/backend/internal/explanation"
	"bia-energy.local/backend/internal/meter"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

type stack struct {
	server *httptest.Server
	client *http.Client
	runs   *analysisrun.Service
	pool   *pgxpool.Pool
}

// newStack serves the real router over real HTTP against PostgreSQL. The
// worker is not started: tests process runs explicitly, so QUEUED and
// COMPLETED states are observed deterministically.
func newStack(t *testing.T, pool *pgxpool.Pool, analyzer analysisrun.Analyzer) *stack {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if analyzer == nil {
		e, err := analysis.New(analysis.DefaultConfig())
		require.NoError(t, err)
		analyzer = e
	}
	explainers := analysisrun.Explainers{Primary: explanation.Deterministic{}, Settings: explanation.DeterministicSettings()}
	runs, err := analysisrun.NewService(pool, analyzer, explainers, analysis.EngineVersion, analysis.DefaultConfig(), logger, analysisrun.Options{})
	require.NoError(t, err)
	authManager, err := auth.NewManager(testAuth, nil)
	require.NoError(t, err)
	srv := httptest.NewServer(NewRouter(Deps{
		Logger: logger, DB: pool, Auth: authManager, Runs: runs,
		Meters: meter.NewService(pool), Anomalies: anomaly.NewService(pool), Dashboard: dashboard.NewService(pool),
	}))
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &stack{server: srv, client: &http.Client{Jar: jar}, runs: runs, pool: pool}
}

func (s *stack) login(t *testing.T) {
	t.Helper()
	resp := s.request(t, http.MethodPost, "/api/v1/auth/login", `{"username":"test-operator","password":"test-password-123"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (s *stack) request(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, s.server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// get decodes a 200 JSON response into out; the DTO types decode source
// times strictly (an offset fails).
func (s *stack) get(t *testing.T, path string, out any) {
	t.Helper()
	resp := s.request(t, http.MethodGet, path, "")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", path, body)
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(out), "%s: %s", path, body)
}

func (s *stack) raw(t *testing.T, path string) map[string]any {
	t.Helper()
	var m map[string]any
	s.get(t, path, &m)
	return m
}

func (s *stack) wantError(t *testing.T, method, path string, status int, code string) {
	t.Helper()
	resp := s.request(t, method, path, "")
	require.Equal(t, status, resp.StatusCode, path)
	var body errorBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body), path)
	assert.Equal(t, code, body.Error.Code, path)
	assert.Equal(t, resp.Header.Get(RequestIDHeader), body.Error.RequestID, path)
}

func (s *stack) analyze(t *testing.T) analyzeDTO {
	t.Helper()
	resp := s.request(t, http.MethodPost, "/api/v1/ai/analyze", "")
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	var body analyzeDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "/api/v1/ai/analysis/"+itoa(body.AnalysisID), resp.Header.Get("Location"))
	return body
}

func (s *stack) process(t *testing.T) {
	t.Helper()
	processed, err := s.runs.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func totalConsumption(t *testing.T, pool *pgxpool.Pool) float64 {
	t.Helper()
	var v float64
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT sum(consumption_kwh)::float8 FROM readings").Scan(&v))
	return v
}

func TestAPI_BeforeAnyAnalysis_TheStateIsHonest(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)
	s.login(t)

	var d dashboardDTO
	s.get(t, "/api/v1/dashboard/summary", &d)
	assert.EqualValues(t, 12, d.Meters)
	assert.EqualValues(t, 4032, d.Readings)
	assert.InDelta(t, totalConsumption(t, db.Pool), d.TotalConsumptionKWh, 1e-6)
	assert.Zero(t, d.Anomalies)
	assert.Zero(t, d.HighPriority)
	assert.Nil(t, d.AggregateConfidence)
	assert.Nil(t, d.LatestAnalysis)
	assert.Nil(t, d.ActiveAnalysis)
	require.NotNil(t, d.Period)

	raw := s.raw(t, "/api/v1/dashboard/summary")
	assert.Nil(t, raw["aggregate_confidence"], "null, never a fabricated value")
	assert.Nil(t, raw["latest_analysis"])
	assert.Equal(t, "2026-09-01T00:00:00", raw["period"].(map[string]any)["first_reading_at"])

	var meters meterListDTO
	s.get(t, "/api/v1/meters", &meters)
	assert.Len(t, meters.Items, 12)
	assert.Nil(t, meters.Analysis)
	for _, m := range meters.Items {
		assert.Nil(t, m.ComputedStatus, "%s is not analyzed yet", m.MeterID)
		assert.Nil(t, m.VariationPct)
	}

	var anomalies anomalyListDTO
	s.get(t, "/api/v1/anomalies", &anomalies)
	assert.Empty(t, anomalies.Items)
	assert.Zero(t, anomalies.Pagination.Total)
}

func TestAPI_AnalysisLifecycleAndCurrentState(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)
	s.login(t)

	first := s.analyze(t)
	assert.True(t, first.Created)
	assert.Equal(t, "QUEUED", first.Status)
	assert.Equal(t, "QUEUED", first.Stage)
	assert.Zero(t, first.Progress)
	again := s.analyze(t)
	assert.False(t, again.Created, "an active run is returned, not duplicated")
	assert.Equal(t, first.AnalysisID, again.AnalysisID)

	var queued runDTO
	s.get(t, "/api/v1/ai/analysis/"+itoa(first.AnalysisID), &queued)
	assert.Equal(t, "QUEUED", queued.Status)
	assert.Nil(t, queued.StartedAt)
	var d dashboardDTO
	s.get(t, "/api/v1/dashboard/summary", &d)
	require.NotNil(t, d.ActiveAnalysis)
	assert.Equal(t, first.AnalysisID, d.ActiveAnalysis.ID)
	assert.Nil(t, d.LatestAnalysis, "a queued run is not the current state")

	s.process(t)

	var done runDTO
	s.get(t, "/api/v1/ai/analysis/"+itoa(first.AnalysisID), &done)
	assert.Equal(t, "COMPLETED", done.Status)
	assert.Equal(t, "COMPLETED", done.Stage)
	assert.Equal(t, 100, done.Progress)
	assert.Equal(t, 4, *done.FindingsCount)
	assert.Equal(t, 2, *done.HighPriorityCount)
	assert.Nil(t, done.Error)
	raw := s.raw(t, "/api/v1/ai/analysis/"+itoa(first.AnalysisID))
	assert.Regexp(t, `^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z$`, raw["completed_at"], "system instants are RFC 3339 UTC")
	assert.IsType(t, float64(0), raw["aggregate_confidence"])

	s.get(t, "/api/v1/dashboard/summary", &d)
	assert.Equal(t, 4, d.Anomalies)
	assert.Equal(t, 2, d.HighPriority)
	require.NotNil(t, d.LatestAnalysis)
	assert.Equal(t, first.AnalysisID, d.LatestAnalysis.ID)
	assert.Equal(t, done.AggregateConfidence, d.AggregateConfidence)
	assert.Nil(t, d.ActiveAnalysis)

	// Run B is queued: the current state stays run A.
	b := s.analyze(t)
	require.True(t, b.Created)
	s.get(t, "/api/v1/dashboard/summary", &d)
	assert.Equal(t, first.AnalysisID, d.LatestAnalysis.ID)
	var anomalies anomalyListDTO
	s.get(t, "/api/v1/anomalies", &anomalies)
	assert.Equal(t, first.AnalysisID, anomalies.Analysis.ID)
	assert.Len(t, anomalies.Items, 4)

	s.process(t)
	s.get(t, "/api/v1/dashboard/summary", &d)
	assert.Equal(t, b.AnalysisID, d.LatestAnalysis.ID, "B is current once COMPLETED")

	s.wantError(t, http.MethodGet, "/api/v1/ai/analysis/999999", http.StatusNotFound, "analysis_not_found")
	s.wantError(t, http.MethodGet, "/api/v1/ai/analysis/abc", http.StatusBadRequest, "invalid_analysis_id")
}

func TestAPI_FailedRun_IsReportedAndDoesNotReplaceTheCurrentState(t *testing.T) {
	db := pgtest.StartSeeded(t)
	good := newStack(t, db.Pool, nil)
	good.login(t)
	a := good.analyze(t)
	good.process(t)

	failing := newStack(t, db.Pool, analyzerFunc(func(context.Context, []analysis.Reading, []analysis.Event) (analysis.Result, error) {
		return analysis.Result{}, errors.New("engine exploded: /secret/path")
	}))
	failing.login(t)
	b := failing.analyze(t)
	failing.process(t)

	var run runDTO
	failing.get(t, "/api/v1/ai/analysis/"+itoa(b.AnalysisID), &run)
	assert.Equal(t, "FAILED", run.Status)
	assert.Equal(t, "FAILED", run.Stage)
	require.NotNil(t, run.Error)
	assert.Equal(t, "analysis_failed", run.Error.Code)
	assert.NotContains(t, run.Error.Message, "/secret/path")
	assert.NotNil(t, run.CompletedAt)
	assert.Nil(t, run.FindingsCount)

	var d dashboardDTO
	good.get(t, "/api/v1/dashboard/summary", &d)
	assert.Equal(t, a.AnalysisID, d.LatestAnalysis.ID)
	assert.Equal(t, 4, d.Anomalies)
}

func TestAPI_ConcurrentAnalyzeRequests_ShareOneRun(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)
	s.login(t)
	for round := range 20 {
		t.Run(strconv.Itoa(round), func(t *testing.T) {
			const callers = 8
			results := make([]analyzeDTO, callers)
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := range callers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					req, _ := http.NewRequest(http.MethodPost, s.server.URL+"/api/v1/ai/analyze", nil)
					resp, err := s.client.Do(req)
					if err != nil {
						return
					}
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusAccepted {
						_ = json.NewDecoder(resp.Body).Decode(&results[i])
					}
				}()
			}
			close(start)
			wg.Wait()

			created := 0
			for _, r := range results {
				require.NotZero(t, r.AnalysisID)
				assert.Equal(t, results[0].AnalysisID, r.AnalysisID)
				if r.Created {
					created++
				}
			}
			assert.Equal(t, 1, created)
			s.process(t)
		})
	}
}

func TestAPI_AnomaliesAfterAnalysis(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)
	s.login(t)
	run := s.analyze(t)
	s.process(t)

	var list anomalyListDTO
	s.get(t, "/api/v1/anomalies", &list)
	require.Len(t, list.Items, 4)
	assert.EqualValues(t, 4, list.Pagination.Total)
	assert.Equal(t, run.AnalysisID, list.Analysis.ID)
	var order []string
	for i, a := range list.Items {
		order = append(order, a.MeterID)
		assert.Equal(t, i+1, a.Priority)
		assert.Equal(t, "OPEN", a.Status)
		assert.Equal(t, run.AnalysisID, a.AnalysisID)
	}
	assert.Equal(t, []string{"M-109", "M-112", "M-104", "M-106"}, order)
	assert.Equal(t, "REAL_ANOMALY", list.Items[0].Type)
	assert.Equal(t, "HIGH", list.Items[0].Severity)
	assert.Equal(t, "FALSE_POSITIVE", list.Items[3].Type)
	assert.Equal(t, "LOW", list.Items[3].Severity)

	filter := func(query string) []string {
		var l anomalyListDTO
		s.get(t, "/api/v1/anomalies?"+query, &l)
		var ids []string
		for _, a := range l.Items {
			ids = append(ids, a.MeterID)
		}
		return ids
	}
	assert.Equal(t, []string{"M-109", "M-112"}, filter("severity=HIGH"))
	assert.Equal(t, []string{"M-112"}, filter("type=DATA_QUALITY"))
	assert.Equal(t, []string{"M-106"}, filter("meter_id=M-106"))
	assert.Empty(t, filter("meter_id=M-101"))
	var page anomalyListDTO
	s.get(t, "/api/v1/anomalies?limit=2&offset=1", &page)
	assert.Equal(t, "M-112", page.Items[0].MeterID)
	assert.Equal(t, "M-104", page.Items[1].MeterID)
	assert.EqualValues(t, 4, page.Pagination.Total)

	var detail anomalyDetailDTO
	s.get(t, "/api/v1/anomalies/"+itoa(list.Items[0].ID), &detail)
	assert.Equal(t, "M-109", detail.MeterID)
	assert.Equal(t, "UNEXPLAINED_PERSISTENT_CONSUMPTION_SHIFT", detail.Rule)
	assert.Equal(t, "INVESTIGATE_METER_AND_INSTALLATION", detail.RecommendedAction)
	assert.Equal(t, 58.0, detail.DurationHours)
	assert.Greater(t, detail.Evidence.Consumption.MedianDeviationPct, 90.0)
	require.Len(t, detail.Evidence.RelatedEvents, 1)
	assert.Equal(t, "UNKNOWN", detail.Evidence.RelatedEvents[0].Type)
	assert.NotEmpty(t, detail.Evidence.Signals)

	raw := s.raw(t, "/api/v1/anomalies/"+itoa(list.Items[0].ID))
	assert.Equal(t, "2026-09-12T14:00:00", raw["started_at"], "source times carry no offset")
	assert.IsType(t, map[string]any{}, raw["evidence"], "evidence is structured JSON, not a string")
	assert.IsType(t, float64(0), raw["confidence"])
	assert.Regexp(t, `Z$`, raw["created_at"])
	signal := raw["evidence"].(map[string]any)["signals"].([]any)[0].(map[string]any)
	assert.Equal(t, "2026-09-12T14:00:00", signal["timestamp"])

	s.wantError(t, http.MethodGet, "/api/v1/anomalies/999999", http.StatusNotFound, "anomaly_not_found")
	s.wantError(t, http.MethodGet, "/api/v1/anomalies/x1", http.StatusBadRequest, "invalid_anomaly_id")
	s.wantError(t, http.MethodGet, "/api/v1/anomalies?severity=CRITICAL", http.StatusBadRequest, "invalid_query")
}

func TestAPI_MetersAfterAnalysis(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)
	s.login(t)
	run := s.analyze(t)
	s.process(t)

	ids := func(query string) []string {
		var l meterListDTO
		s.get(t, "/api/v1/meters?"+query, &l)
		var out []string
		for _, m := range l.Items {
			out = append(out, m.MeterID)
		}
		return out
	}

	var all meterListDTO
	s.get(t, "/api/v1/meters", &all)
	require.Len(t, all.Items, 12)
	assert.EqualValues(t, 12, all.Pagination.Total)
	assert.Equal(t, run.AnalysisID, all.Analysis.ID)
	byID := map[string]meterSummaryDTO{}
	for _, m := range all.Items {
		byID[m.MeterID] = m
	}
	assert.Equal(t, "CRITICAL", *byID["M-109"].ComputedStatus)
	assert.Equal(t, "ALERT", *byID["M-112"].ComputedStatus)
	assert.Equal(t, "ALERT", *byID["M-104"].ComputedStatus)
	assert.Equal(t, "OK", *byID["M-106"].ComputedStatus, "a false positive does not raise the meter status")
	assert.Equal(t, "FALSE_POSITIVE", *byID["M-106"].AnomalyType)
	assert.Equal(t, "OK", *byID["M-101"].ComputedStatus)
	assert.Nil(t, byID["M-101"].VariationPct, "no finding: null, not 0%")
	assert.Nil(t, byID["M-101"].Severity)
	assert.Greater(t, *byID["M-109"].VariationPct, 90.0)

	assert.Equal(t, []string{"M-109"}, ids("status=CRITICAL"))
	assert.Equal(t, []string{"M-104", "M-112"}, ids("status=ALERT"))
	assert.Len(t, ids("status=OK"), 9)
	assert.Equal(t, []string{"M-110", "M-111", "M-112"}, ids("search=m-11"), "case-insensitive partial match")
	assert.Empty(t, ids("search=%25"), "wildcards are literal text")
	assert.Equal(t, []string{"M-109", "M-106", "M-104", "M-112"}, ids("sort=variation")[:4], "largest variation magnitude first")
	assert.Equal(t, []string{"M-109", "M-112", "M-104", "M-106"}, ids("sort=severity")[:4])
	assert.Equal(t, []string{"M-112", "M-111"}, ids("sort=meter_id&order=desc&limit=2"))

	var byConsumption meterListDTO
	s.get(t, "/api/v1/meters?sort=consumption", &byConsumption)
	for i := 1; i < len(byConsumption.Items); i++ {
		assert.GreaterOrEqual(t, byConsumption.Items[i-1].TotalConsumptionKWh, byConsumption.Items[i].TotalConsumptionKWh)
	}
	var page meterListDTO
	s.get(t, "/api/v1/meters?limit=5&offset=10", &page)
	assert.Len(t, page.Items, 2)
	assert.EqualValues(t, 12, page.Pagination.Total)

	var detail meterDetailDTO
	s.get(t, "/api/v1/meters/M-109", &detail)
	assert.Equal(t, "CRITICAL", *detail.ComputedStatus)
	assert.EqualValues(t, 336, detail.ReadingsCount)
	require.NotNil(t, detail.CurrentFinding)
	assert.Equal(t, "REAL_ANOMALY", detail.CurrentFinding.Type)
	assert.Equal(t, 1, detail.CurrentFinding.Priority)
	assert.Equal(t, 1, detail.FindingsCount)
	assert.Equal(t, run.AnalysisID, detail.Analysis.ID)
	rawDetail := s.raw(t, "/api/v1/meters/M-109")
	assert.Equal(t, "2026-09-14T23:00:00", rawDetail["period"].(map[string]any)["last_reading_at"])

	var quiet meterDetailDTO
	s.get(t, "/api/v1/meters/M-101", &quiet)
	assert.Nil(t, quiet.CurrentFinding)
	assert.Equal(t, "OK", *quiet.ComputedStatus)

	s.wantError(t, http.MethodGet, "/api/v1/meters/M-999", http.StatusNotFound, "meter_not_found")
	s.wantError(t, http.MethodGet, "/api/v1/meters?sort=drop", http.StatusBadRequest, "invalid_query")
	s.wantError(t, http.MethodGet, "/api/v1/meters?limit=1000", http.StatusBadRequest, "invalid_query")
}

func TestAPI_Readings(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)
	s.login(t)

	var all readingsDTO
	s.get(t, "/api/v1/meters/M-109/readings", &all)
	require.Len(t, all.Items, 336)
	assert.False(t, all.HasMore)
	first := all.Items[0]
	assert.Equal(t, "2026-09-01T00:00:00", jsonString(t, first.Timestamp))
	assert.Equal(t, "OK", first.SourceStatus, "the source status column, unchanged")

	var window readingsDTO
	s.get(t, "/api/v1/meters/M-109/readings?from=2026-09-12T14:00:00&to=2026-09-12T16:00:00", &window)
	require.Len(t, window.Items, 2, "half-open range [from, to)")
	assert.Equal(t, "2026-09-12T14:00:00", jsonString(t, window.Items[0].Timestamp))
	assert.Equal(t, "2026-09-12T15:00:00", jsonString(t, window.Items[1].Timestamp))

	var limited readingsDTO
	s.get(t, "/api/v1/meters/M-109/readings?limit=10", &limited)
	assert.Len(t, limited.Items, 10)
	assert.True(t, limited.HasMore)

	var empty readingsDTO
	s.get(t, "/api/v1/meters/M-109/readings?from=2026-09-12T14:00:00&to=2026-09-12T14:00:00", &empty)
	assert.Empty(t, empty.Items)

	// Values equal the source CSV row (M-109 is loaded from data/input).
	var value float64
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		"SELECT voltage_v::float8 FROM readings WHERE meter_id = 'M-109' AND reading_timestamp = '2026-09-12 14:00:00'").Scan(&value))
	assert.Equal(t, value, window.Items[0].VoltageV)

	s.wantError(t, http.MethodGet, "/api/v1/meters/M-999/readings", http.StatusNotFound, "meter_not_found")
	s.wantError(t, http.MethodGet, "/api/v1/meters/M-109/readings?from=2026-09-13T00:00:00&to=2026-09-12T00:00:00", http.StatusBadRequest, "invalid_range")
	s.wantError(t, http.MethodGet, "/api/v1/meters/M-109/readings?from=2026-09-12T14:00:00Z", http.StatusBadRequest, "invalid_timestamp")
}

func jsonString(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var s string
	require.NoError(t, json.Unmarshal(b, &s))
	return s
}

func TestAPI_AuthenticationFlow(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)

	s.wantError(t, http.MethodGet, "/api/v1/dashboard/summary", http.StatusUnauthorized, "unauthorized")
	resp := s.request(t, http.MethodPost, "/api/v1/auth/login", `{"username":"test-operator","password":"wrong-password"}`)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	s.login(t)
	var sess sessionDTO
	s.get(t, "/api/v1/auth/session", &sess)
	assert.Equal(t, "test-operator", sess.Username)
	var d dashboardDTO
	s.get(t, "/api/v1/dashboard/summary", &d)

	require.Equal(t, http.StatusNoContent, s.request(t, http.MethodPost, "/api/v1/auth/logout", "").StatusCode)
	s.wantError(t, http.MethodGet, "/api/v1/dashboard/summary", http.StatusUnauthorized, "unauthorized")
	assert.Equal(t, http.StatusOK, s.request(t, http.MethodGet, "/healthz", "").StatusCode)
	assert.Equal(t, http.StatusOK, s.request(t, http.MethodGet, "/readyz", "").StatusCode)
}

type analyzerFunc func(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error)

func (f analyzerFunc) Analyze(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error) {
	return f(ctx, r, e)
}

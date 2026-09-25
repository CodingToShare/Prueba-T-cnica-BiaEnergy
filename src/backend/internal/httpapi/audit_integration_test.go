//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
	"github.com/stretchr/testify/require"
)

func TestAPI_AuditCurrentStateAcrossFailureAndHistory(t *testing.T) {
	db := pgtest.StartSeeded(t)
	real, err := analysis.New(analysis.DefaultConfig())
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	calls := 0
	s := newStack(t, db.Pool, analyzerFunc(func(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error) {
		calls++
		if calls == 2 {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return analysis.Result{}, ctx.Err()
			}
			return analysis.Result{}, errors.New(`postgres://secret-host/private-db C:\private\path`)
		}
		return real.Analyze(ctx, r, e)
	}))
	s.login(t)
	a := s.analyze(t)
	s.process(t)
	check := func(id int64) {
		var d dashboardDTO
		var m meterListDTO
		var an anomalyListDTO
		s.get(t, "/api/v1/dashboard/summary", &d)
		s.get(t, "/api/v1/meters", &m)
		s.get(t, "/api/v1/anomalies", &an)
		require.Equal(t, id, d.LatestAnalysis.ID)
		require.Equal(t, id, m.Analysis.ID)
		require.Equal(t, id, an.Analysis.ID)
		require.Len(t, an.Items, 4)
	}
	b := s.analyze(t)
	check(a.AnalysisID)
	done := make(chan error, 1)
	go func() { _, err := s.runs.ProcessNext(context.Background()); done <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not start")
	}
	check(a.AnalysisID)
	once.Do(func() { close(release) })
	require.NoError(t, <-done)
	check(a.AnalysisID)
	var failed runDTO
	s.get(t, "/api/v1/ai/analysis/"+itoa(b.AnalysisID), &failed)
	require.Equal(t, "FAILED", failed.Status)
	require.Equal(t, analysisrun.FailureMessage(analysisrun.CodeAnalysisFailed), failed.Error.Message)
	c := s.analyze(t)
	s.process(t)
	check(c.AnalysisID)
	var historic runDTO
	s.get(t, "/api/v1/ai/analysis/"+itoa(a.AnalysisID), &historic)
	require.Equal(t, "COMPLETED", historic.Status)
	var count int
	require.NoError(t, db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM anomalies WHERE analysis_run_id=$1", a.AnalysisID).Scan(&count))
	require.Equal(t, 4, count)
}

func TestAPI_AuditFiltersRangesAndEvidence(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, nil)
	s.login(t)
	for _, status := range []string{"OK", "ALERT", "CRITICAL"} {
		var page meterListDTO
		s.get(t, "/api/v1/meters?status="+status, &page)
		require.Empty(t, page.Items)
		require.Zero(t, page.Pagination.Total)
	}
	var unicodePage meterListDTO
	s.get(t, "/api/v1/meters?search="+url.QueryEscape(strings.Repeat("é", 64)), &unicodePage)
	require.Empty(t, unicodePage.Items)
	s.wantError(t, http.MethodGet, "/api/v1/meters?search="+url.QueryEscape(strings.Repeat("é", 65)), 400, "invalid_query")
	for _, id := range []string{`odd%meter`, `odd_meter`, `odd\meter`} {
		_, err := db.Pool.Exec(context.Background(), "INSERT INTO meters(meter_id) VALUES ($1)", id)
		require.NoError(t, err)
	}
	for _, needle := range []string{"%", "_", `\`} {
		var page meterListDTO
		s.get(t, "/api/v1/meters?search="+url.QueryEscape(needle), &page)
		require.Len(t, page.Items, 1)
		require.Contains(t, page.Items[0].MeterID, needle)
	}
	var mixed meterListDTO
	s.get(t, "/api/v1/meters?search=oDd", &mixed)
	require.Len(t, mixed.Items, 3)
	// Test-only synthetic search rows are removed before supplied-data analysis.
	_, err := db.Pool.Exec(context.Background(), "DELETE FROM meters WHERE meter_id = ANY($1)", []string{`odd%meter`, `odd_meter`, `odd\meter`})
	require.NoError(t, err)
	s.analyze(t)
	s.process(t)
	var list anomalyListDTO
	s.get(t, "/api/v1/anomalies", &list)
	sum := 0.0
	for _, a := range list.Items {
		sum += a.Confidence
		var detail anomalyDetailDTO
		s.get(t, "/api/v1/anomalies/"+itoa(a.ID), &detail)
		var raw []byte
		require.NoError(t, db.Pool.QueryRow(context.Background(), "SELECT evidence FROM anomalies WHERE id=$1", a.ID).Scan(&raw))
		var persisted analysisrun.Evidence
		require.NoError(t, json.Unmarshal(raw, &persisted))
		require.Equal(t, persisted, detail.Evidence)
		require.Equal(t, 1, detail.Evidence.SchemaVersion)
		t.Logf("priority=%d meter=%s type=%s severity=%s confidence=%.12f", a.Priority, a.MeterID, a.Type, a.Severity, a.Confidence)
	}
	var d dashboardDTO
	s.get(t, "/api/v1/dashboard/summary", &d)
	require.InDelta(t, sum/4, *d.AggregateConfidence, 1e-12)
	require.InDelta(t, 155250.85, d.TotalConsumptionKWh, 1e-6)
	for _, tc := range []struct {
		query string
		count int
	}{{"severity=HIGH&type=DATA_QUALITY&meter_id=M-112", 1}, {"severity=LOW&type=REAL_ANOMALY", 0}} {
		var page anomalyListDTO
		s.get(t, "/api/v1/anomalies?"+tc.query, &page)
		require.Len(t, page.Items, tc.count)
		require.EqualValues(t, tc.count, page.Pagination.Total)
	}
	for _, query := range []string{"limit=100&offset=1000000", "limit=1&offset=999"} {
		var page meterListDTO
		s.get(t, "/api/v1/meters?"+query, &page)
		require.Empty(t, page.Items)
		require.EqualValues(t, 12, page.Pagination.Total)
	}
	for _, order := range []string{"asc", "desc"} {
		var page meterListDTO
		s.get(t, "/api/v1/meters?sort=variation&order="+order, &page)
		values := []float64{}
		nullSeen := false
		for _, m := range page.Items {
			if m.VariationPct == nil {
				nullSeen = true
				continue
			}
			require.False(t, nullSeen)
			values = append(values, math.Abs(*m.VariationPct))
		}
		for i := 1; i < len(values); i++ {
			if order == "asc" {
				require.LessOrEqual(t, values[i-1], values[i])
			} else {
				require.GreaterOrEqual(t, values[i-1], values[i])
			}
		}
	}
	for _, tc := range []struct {
		query string
		count int
	}{{"to=2026-09-01T02:00:00", 2}, {"from=2026-09-14T22:00:00", 2}, {"from=2030-01-01T00:00:00", 0}} {
		var r readingsDTO
		s.get(t, "/api/v1/meters/M-109/readings?"+tc.query, &r)
		require.Len(t, r.Items, tc.count)
	}
	s.wantError(t, http.MethodGet, "/api/v1/meters/M-109/readings?from=2026-09-12T14:00:00.123", 400, "invalid_timestamp")
	_, err = db.Pool.Exec(context.Background(), `INSERT INTO readings(meter_id,reading_timestamp,consumption_kwh,voltage_v,current_a,power_factor,source_status) SELECT 'M-109', timestamp '2030-01-01' + n * interval '1 hour', 1, 220, 1, 0.9, 'OK' FROM generate_series(0,1004) n`)
	require.NoError(t, err)
	var capped readingsDTO
	s.get(t, "/api/v1/meters/M-109/readings?from=2030-01-01T00:00:00", &capped)
	require.Len(t, capped.Items, 1000)
	require.True(t, capped.HasMore)
}

func TestAPI_AuditZeroFindingConfidenceIsNull(t *testing.T) {
	db := pgtest.StartSeeded(t)
	s := newStack(t, db.Pool, analyzerFunc(func(context.Context, []analysis.Reading, []analysis.Event) (analysis.Result, error) {
		return analysis.Result{}, nil
	}))
	s.login(t)
	s.analyze(t)
	s.process(t)
	var d dashboardDTO
	s.get(t, "/api/v1/dashboard/summary", &d)
	require.NotNil(t, d.LatestAnalysis)
	require.Zero(t, d.Anomalies)
	require.Zero(t, d.HighPriority)
	require.Nil(t, d.AggregateConfidence)
}

package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scrape(t *testing.T, m *Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return string(body)
}

func histogramSample(t *testing.T, m *Metrics, name string) (uint64, float64) {
	t.Helper()
	families, err := m.Gatherer().Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		samples := family.GetMetric()
		require.Len(t, samples, 1)
		histogram := samples[0].GetHistogram()
		require.NotNil(t, histogram)
		return histogram.GetSampleCount(), histogram.GetSampleSum()
	}
	require.FailNow(t, "histogram not found", name)
	return 0, 0
}

func TestNew_EachInstanceOwnsItsRegistry(t *testing.T) {
	// Two instances never collide (no global default registry), so tests
	// and components can build their own.
	a, b := New(), New()
	a.RunStarted()

	active, ok := Value(a.Gatherer(), "bia_analysis_runs_active", nil)
	require.True(t, ok)
	assert.Equal(t, 1.0, active)
	active, _ = Value(b.Gatherer(), "bia_analysis_runs_active", nil)
	assert.Equal(t, 0.0, active)
}

func TestNilMetrics_RecordNothingAndDoNotPanic(t *testing.T) {
	var m *Metrics
	m.ObserveHTTP("GET", "/healthz", 200, time.Millisecond)
	m.RunStarted()
	m.RunStopped()
	m.RunFinished("completed", time.Second, 4)
	m.Explanation("deterministic", OutcomeGenerated, "", time.Microsecond)
}

func TestRunStopped_ClearsActiveWithoutClaimingATerminalStatus(t *testing.T) {
	m := New()
	m.RunStarted()
	m.RunStopped()

	active, ok := Value(m.Gatherer(), "bia_analysis_runs_active", nil)
	require.True(t, ok)
	assert.Equal(t, 0.0, active)
	_, ok = Value(m.Gatherer(), "bia_analysis_runs_total", map[string]string{"status": "failed"})
	assert.False(t, ok)
}

func TestRecording_ExposesTheExpectedFamiliesAndSamples(t *testing.T) {
	m := New()
	m.ObserveHTTP("GET", "/api/v1/meters/{meterId}", 401, 3*time.Millisecond)
	m.ObserveHTTP("GET", "/api/v1/meters/{meterId}", 200, 3*time.Millisecond)
	m.RunStarted()
	m.RunFinished("completed", 250*time.Millisecond, 4)
	m.RunStarted()
	m.RunFinished("failed", time.Second, 0)
	m.Explanation("ollama", OutcomeGenerated, "", 8*time.Second)
	m.Explanation("ollama", OutcomeFallback, "timeout", 60*time.Second)

	g := m.Gatherer()
	get := func(name string, labels map[string]string) float64 {
		t.Helper()
		v, ok := Value(g, name, labels)
		require.True(t, ok, "%s %v", name, labels)
		return v
	}
	assert.Equal(t, 1.0, get("bia_http_requests_total", map[string]string{"method": "GET", "route": "/api/v1/meters/{meterId}", "status_class": "4xx"}))
	assert.Equal(t, 1.0, get("bia_http_requests_total", map[string]string{"status_class": "2xx"}))
	assert.Equal(t, 2.0, get("bia_http_request_duration_seconds", map[string]string{"route": "/api/v1/meters/{meterId}"}))
	assert.Equal(t, 1.0, get("bia_analysis_runs_total", map[string]string{"status": "completed"}))
	assert.Equal(t, 1.0, get("bia_analysis_runs_total", map[string]string{"status": "failed"}))
	findingsCount, findingsSum := histogramSample(t, m, "bia_analysis_findings")
	assert.Equal(t, uint64(1), findingsCount, "one completed run was observed")
	assert.Equal(t, 4.0, findingsSum, "the completed run produced four findings")
	assert.Equal(t, 0.0, get("bia_analysis_runs_active", nil))
	assert.Equal(t, 1.0, get("bia_explanation_generation_total", map[string]string{"provider": "ollama", "outcome": "generated"}))
	assert.Equal(t, 1.0, get("bia_explanation_generation_total", map[string]string{"provider": "ollama", "outcome": "fallback"}))
	assert.Equal(t, 1.0, get("bia_explanation_fallback_total", map[string]string{"fallback_code": "timeout"}))
	assert.Equal(t, 2.0, get("bia_explanation_generation_duration_seconds", map[string]string{"provider": "ollama"}))

	text := scrape(t, m)
	for _, family := range []string{"bia_http_requests_total", "bia_analysis_run_duration_seconds", "go_goroutines"} {
		assert.Contains(t, text, "# TYPE "+family)
	}
}

func TestUnavailableExplanation_IsNotCountedAsAFallback(t *testing.T) {
	m := New()
	m.Explanation("ollama", OutcomeUnavailable, "provider_unavailable", time.Second)
	_, ok := Value(m.Gatherer(), "bia_explanation_fallback_total", map[string]string{"fallback_code": "provider_unavailable"})
	assert.False(t, ok)
}

func TestStatusClass_IsBounded(t *testing.T) {
	assert.Equal(t, "2xx", statusClass(204))
	assert.Equal(t, "5xx", statusClass(503))
	assert.Equal(t, "other", statusClass(0))
	assert.Equal(t, "other", statusClass(999))
	assert.False(t, strings.Contains(statusClass(404), "404"))
}

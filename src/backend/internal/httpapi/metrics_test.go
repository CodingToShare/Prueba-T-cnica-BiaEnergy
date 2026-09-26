package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/platform/metrics"
)

func TestMetrics_ArePublicAndUseRouteTemplatesNotIdentifiers(t *testing.T) {
	f := newFixture(t, pinger{})
	f.do(http.MethodGet, "/api/v1/meters/SECRET-METER-42?from=2030-01-01T00:00:00", "")
	f.do(http.MethodGet, "/api/v1/anomalies/987654", "")
	f.do(http.MethodGet, "/no/such/path/123456", "")
	f.do("PROPFIND", "/healthz", "")

	w := f.do(http.MethodGet, "/metrics", "")
	require.Equal(t, http.StatusOK, w.Code, "metrics are public like health and readiness")
	body := w.Body.String()
	assert.Contains(t, body, "# TYPE bia_http_requests_total counter")

	g := f.metrics.Gatherer()
	value := func(labels map[string]string) float64 {
		t.Helper()
		v, ok := metrics.Value(g, "bia_http_requests_total", labels)
		require.True(t, ok, "%v", labels)
		return v
	}
	assert.Equal(t, 1.0, value(map[string]string{"method": "GET", "route": "/api/v1/meters/{meterId}", "status_class": "4xx"}))
	assert.Equal(t, 1.0, value(map[string]string{"route": "/api/v1/anomalies/{id}"}))
	assert.Equal(t, 1.0, value(map[string]string{"route": "unmatched", "status_class": "4xx"}))
	assert.Equal(t, 1.0, value(map[string]string{"method": "OTHER"}))

	for _, leaked := range []string{"SECRET-METER-42", "987654", "123456", "from=", "PROPFIND", "request_id"} {
		assert.NotContains(t, body, leaked, "no identifier, raw path, query or unbounded method is a label")
	}
}

//go:build integration

package functional

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

// client talks to the running API over real HTTP with a cookie jar, the way
// a browser would. Responses are decoded into plain maps: the test knows
// only the public contract.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (c *client) call(method, path, body string) (int, http.Header, map[string]any) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	require.NoError(c.t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	var decoded map[string]any
	if len(raw) > 0 {
		require.NoError(c.t, json.Unmarshal(raw, &decoded), "%s %s: %s", method, path, raw)
	}
	return resp.StatusCode, resp.Header, decoded
}

func (c *client) ok(path string) map[string]any {
	c.t.Helper()
	code, _, body := c.call(http.MethodGet, path, "")
	require.Equal(c.t, http.StatusOK, code, "GET %s: %v", path, body)
	return body
}

func (c *client) unauthorized(path string) {
	c.t.Helper()
	code, _, body := c.call(http.MethodGet, path, "")
	require.Equal(c.t, http.StatusUnauthorized, code, path)
	assert.Equal(c.t, "unauthorized", body["error"].(map[string]any)["code"])
}

func items(body map[string]any) []map[string]any {
	var out []map[string]any
	for _, it := range body["items"].([]any) {
		out = append(out, it.(map[string]any))
	}
	return out
}

func TestBlackBox_AuthenticatedAnalysisGoldenPath(t *testing.T) {
	bin := buildCommands(t)
	db := pgtest.StartWithPassword(t, auditPassword)
	env := apiEnv(db.URL)
	var output syncBuffer

	runToCompletion(t, bin["migrate"], env, &output, "up")
	runToCompletion(t, bin["seed"], env, &output)

	api := exec.Command(bin["api"])
	api.Env = env
	api.Stdout = &output
	api.Stderr = &output
	require.NoError(t, api.Start())
	done := make(chan struct{})
	var waitErr error
	go func() { waitErr = api.Wait(); close(done) }()
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			_ = api.Process.Kill()
			<-done
		}
	})

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	c := &client{t: t, base: "http://" + awaitListening(t, &output), http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}

	// Operational endpoints are public; the product API is not.
	assert.Equal(t, http.StatusOK, status(c.base+"/healthz"))
	assert.Equal(t, http.StatusOK, status(c.base+"/readyz"))
	c.unauthorized("/api/v1/dashboard/summary")

	code, _, body := c.call(http.MethodPost, "/api/v1/auth/login", `{"username":"`+demoUser+`","password":"wrong-password"}`)
	require.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "invalid_credentials", body["error"].(map[string]any)["code"])
	code, _, body = c.call(http.MethodPost, "/api/v1/auth/login", `{"username":"`+demoUser+`","password":"`+demoPassword+`"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, body["authenticated"])
	sessionCookies := jar.Cookies(mustURL(t, c.base))
	assert.Equal(t, demoUser, c.ok("/api/v1/auth/session")["username"])

	// Before any analysis: source facts only, no claimed results.
	var sourceTotal float64
	require.NoError(t, db.Pool.QueryRow(context.Background(), "SELECT sum(consumption_kwh)::float8 FROM readings").Scan(&sourceTotal))
	file, err := os.Open(filepath.Join("..", "..", "..", "data", "input", "readings.csv"))
	require.NoError(t, err)
	rows, err := csv.NewReader(file).ReadAll()
	file.Close()
	require.NoError(t, err)
	var csvTotal float64
	for _, row := range rows[1:] {
		value, err := strconv.ParseFloat(row[2], 64)
		require.NoError(t, err)
		csvTotal += value
	}
	assert.InDelta(t, 155250.85, csvTotal, 1e-6)
	assert.InDelta(t, csvTotal, sourceTotal, 1e-6)
	before := c.ok("/api/v1/dashboard/summary")
	assert.EqualValues(t, 12, before["meters"])
	assert.InDelta(t, sourceTotal, before["total_consumption_kwh"], 1e-6)
	assert.EqualValues(t, 0, before["anomalies"])
	assert.EqualValues(t, 0, before["high_priority"])
	assert.Nil(t, before["aggregate_confidence"])
	assert.Nil(t, before["latest_analysis"])

	// Trigger the analysis; the request does not wait for it.
	code, header, run := c.call(http.MethodPost, "/api/v1/ai/analyze", "")
	require.Equal(t, http.StatusAccepted, code)
	id := strconv.FormatInt(int64(run["analysis_id"].(float64)), 10)
	assert.Equal(t, "/api/v1/ai/analysis/"+id, header.Get("Location"))
	assert.Equal(t, true, run["created"])

	var final map[string]any
	eventually(t, 60*time.Second, "analysis reaches a terminal state", func() bool {
		final = c.ok("/api/v1/ai/analysis/" + id)
		return final["status"] == "COMPLETED" || final["status"] == "FAILED"
	})
	require.Equal(t, "COMPLETED", final["status"], "run: %v", final)
	assert.Equal(t, "COMPLETED", final["stage"])
	assert.EqualValues(t, 100, final["progress"])
	assert.EqualValues(t, 4, final["findings_count"])
	assert.EqualValues(t, 2, final["high_priority_count"])
	assert.EqualValues(t, 4032, final["readings_count"])
	assert.Regexp(t, `Z$`, final["completed_at"])

	// Anomalies in priority order.
	anomalies := c.ok("/api/v1/anomalies")
	list := items(anomalies)
	require.Len(t, list, 4)
	var order []string
	for _, a := range list {
		order = append(order, a["meter_id"].(string))
		t.Logf("priority=%v meter=%v type=%v severity=%v confidence=%.12f", a["priority"], a["meter_id"], a["type"], a["severity"], a["confidence"])
	}
	assert.Equal(t, []string{"M-109", "M-112", "M-104", "M-106"}, order)

	top := c.ok("/api/v1/anomalies/" + strconv.FormatInt(int64(list[0]["id"].(float64)), 10))
	assert.Equal(t, "REAL_ANOMALY", top["type"])
	assert.Equal(t, "HIGH", top["severity"])
	assert.Equal(t, "INVESTIGATE_METER_AND_INSTALLATION", top["recommended_action"])
	assert.Equal(t, "2026-09-12T14:00:00", top["started_at"])
	evidence := top["evidence"].(map[string]any)
	assert.Greater(t, evidence["consumption"].(map[string]any)["median_deviation_pct"].(float64), 90.0)
	corroborating := map[string]bool{}
	for _, m := range evidence["metrics"].([]any) {
		metric := m.(map[string]any)
		corroborating[metric["metric"].(string)] = metric["corroborates"].(bool)
	}
	assert.True(t, corroborating["current_a"])
	assert.True(t, corroborating["power_factor"])
	events := evidence["related_events"].([]any)
	require.Len(t, events, 1)
	assert.Equal(t, "UNKNOWN", events[0].(map[string]any)["type"])
	assert.Equal(t, "CONTEXT", events[0].(map[string]any)["role"])

	// Meters: computed status from the analysis, source status untouched.
	meters := c.ok("/api/v1/meters")
	statuses := map[string]any{}
	for _, m := range items(meters) {
		statuses[m["meter_id"].(string)] = m["computed_status"]
	}
	assert.Len(t, statuses, 12)
	assert.Equal(t, "CRITICAL", statuses["M-109"])
	assert.Equal(t, "ALERT", statuses["M-112"])
	assert.Equal(t, "ALERT", statuses["M-104"])
	assert.Equal(t, "OK", statuses["M-106"])
	detail := c.ok("/api/v1/meters/M-109")
	assert.Equal(t, "CRITICAL", detail["computed_status"])
	assert.Equal(t, "REAL_ANOMALY", detail["current_finding"].(map[string]any)["type"])
	history := c.ok("/api/v1/meters/M-109/readings")
	readings := items(history)
	require.Len(t, readings, 336)
	assert.Equal(t, "OK", readings[len(readings)-1]["source_status"], "source status stays the CSV value while the computed status is CRITICAL")
	assert.Equal(t, "2026-09-14T23:00:00", readings[len(readings)-1]["timestamp"])

	after := c.ok("/api/v1/dashboard/summary")
	assert.EqualValues(t, 12, after["meters"])
	assert.InDelta(t, sourceTotal, after["total_consumption_kwh"], 1e-6)
	assert.EqualValues(t, 4, after["anomalies"])
	assert.EqualValues(t, 2, after["high_priority"])
	assert.Equal(t, final["aggregate_confidence"], after["aggregate_confidence"])
	t.Logf("CSV/SQL/API consumption=%.2f aggregate_confidence=%.12f", sourceTotal, after["aggregate_confidence"])
	latest := after["latest_analysis"].(map[string]any)
	assert.EqualValues(t, run["analysis_id"], latest["id"])
	assert.Equal(t, "COMPLETED", latest["status"])

	// Logout ends the session.
	code, _, _ = c.call(http.MethodPost, "/api/v1/auth/logout", "")
	require.Equal(t, http.StatusNoContent, code)
	c.unauthorized("/api/v1/anomalies")

	if runtime.GOOS == "windows" {
		// Windows cannot deliver SIGTERM to a child process from a test;
		// graceful shutdown (worker included) is verified in-process.
		require.NoError(t, api.Process.Kill())
		<-done
	} else {
		require.NoError(t, api.Process.Signal(syscall.SIGTERM))
		select {
		case <-done:
			require.NoError(t, waitErr, "api must exit cleanly on SIGTERM")
		case <-time.After(15 * time.Second):
			t.Fatal("api did not exit within 15s of SIGTERM")
		}
		assert.Contains(t, output.String(), "analysis worker stopped")
		assert.Contains(t, output.String(), "shutdown complete")
	}

	logs := output.String()
	assert.Contains(t, logs, `"msg":"analysis completed"`)
	assert.Contains(t, logs, `"analysis_id":`+id)
	assertNoSecrets(t, logs)
	for _, cookie := range sessionCookies {
		assert.NotContains(t, logs, cookie.Value)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

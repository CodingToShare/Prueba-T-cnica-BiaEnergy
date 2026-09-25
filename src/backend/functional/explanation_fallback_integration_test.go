//go:build integration

package functional

import (
	"net"
	"net/http"
	"net/http/cookiejar"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

// The compiled API configured for Ollama while nothing listens at the
// configured address: the analysis still completes with the same findings,
// and every explanation is the deterministic fallback with truthful
// provenance. No model is needed.
func TestBlackBox_OllamaUnavailable_AnalysisCompletesWithDeterministicFallback(t *testing.T) {
	bin := buildCommands(t)
	db := pgtest.StartWithPassword(t, auditPassword)
	env := apiEnv(db.URL)
	var output syncBuffer
	runToCompletion(t, bin["migrate"], env, &output, "up")
	runToCompletion(t, bin["seed"], env, &output)

	// Reserve a free port, then close it: connections there are refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	unreachable := "http://" + l.Addr().String()
	require.NoError(t, l.Close())

	api := exec.Command(bin["api"])
	api.Env = append(env,
		"EXPLANATION_PROVIDER=ollama",
		"OLLAMA_BASE_URL="+unreachable,
		"OLLAMA_MODEL=llama3.2:3b",
		"OLLAMA_TIMEOUT=5s",
	)
	api.Stdout = &output
	api.Stderr = &output
	require.NoError(t, api.Start())
	done := make(chan struct{})
	go func() { _ = api.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = api.Process.Kill()
		<-done
	})

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	c := &client{t: t, base: "http://" + awaitListening(t, &output), http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
	code, _, _ := c.call(http.MethodPost, "/api/v1/auth/login", `{"username":"`+demoUser+`","password":"`+demoPassword+`"}`)
	require.Equal(t, http.StatusOK, code)

	code, _, run := c.call(http.MethodPost, "/api/v1/ai/analyze", "")
	require.Equal(t, http.StatusAccepted, code)
	id := strconv.FormatInt(int64(run["analysis_id"].(float64)), 10)
	var final map[string]any
	eventually(t, 90*time.Second, "analysis reaches a terminal state", func() bool {
		final = c.ok("/api/v1/ai/analysis/" + id)
		return final["status"] == "COMPLETED" || final["status"] == "FAILED"
	})
	require.Equal(t, "COMPLETED", final["status"], "an unavailable explanation provider must not fail the run: %v", final)
	assert.EqualValues(t, 4, final["findings_count"])
	assert.EqualValues(t, 2, final["high_priority_count"])

	list := items(c.ok("/api/v1/anomalies"))
	require.Len(t, list, 4)
	want := []struct{ meter, kind, severity string }{
		{"M-109", "REAL_ANOMALY", "HIGH"},
		{"M-112", "DATA_QUALITY", "HIGH"},
		{"M-104", "EXPLAINABLE_ANOMALY", "MEDIUM"},
		{"M-106", "FALSE_POSITIVE", "LOW"},
	}
	for i, a := range list {
		assert.Equal(t, want[i].meter, a["meter_id"])
		assert.Equal(t, want[i].kind, a["type"])
		assert.Equal(t, want[i].severity, a["severity"])

		detail := c.ok("/api/v1/anomalies/" + strconv.FormatInt(int64(a["id"].(float64)), 10))
		explanation := detail["explanation"].(map[string]any)
		assert.Equal(t, "DETERMINISTIC", explanation["source"], a["meter_id"])
		assert.Equal(t, true, explanation["fallback_used"])
		assert.Nil(t, explanation["model"], "fallback text is not attributed to the model")
		assert.Equal(t, "evidence-template-v1", explanation["prompt_version"])
		assert.NotEmpty(t, explanation["summary"])
		assert.NotContains(t, detail, "fallback_code", "the sanitized code stays internal")
	}

	logs := output.String()
	assert.Contains(t, logs, `"msg":"explanation provider configured"`)
	assert.Contains(t, logs, `"msg":"explanation fallback used"`)
	assert.Contains(t, logs, `"fallback_code":"provider_unavailable"`)
	assert.NotContains(t, logs, "connection refused", "raw provider errors are not logged")
	assertNoSecrets(t, logs)
}

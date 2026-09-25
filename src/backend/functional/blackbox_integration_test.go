//go:build integration

// Package functional holds black-box tests: the real commands are compiled
// and run as separate processes against a disposable PostgreSQL container,
// and the API is exercised over real HTTP. Nothing is called in-process.
package functional

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

// Fake credentials; the tests assert they never appear in any process output.
const (
	auditPassword  = "SUPER_SECRET_AUDIT_VALUE"
	demoUser       = "blackbox-operator"
	demoPassword   = "BLACKBOX_DEMO_PASSWORD_VALUE"
	demoSigningKey = "BLACKBOX_SIGNING_KEY_VALUE_0123456789abcdef"
)

// apiEnv is the process environment of every command: database, logging and
// the demo authentication (Secure cookies off: the tests use plain HTTP).
func apiEnv(databaseURL string) []string {
	return processEnv(map[string]string{
		"DATABASE_URL":          databaseURL,
		"HTTP_ADDR":             "127.0.0.1:0",
		"LOG_LEVEL":             "info",
		"DEMO_AUTH_USERNAME":    demoUser,
		"DEMO_AUTH_PASSWORD":    demoPassword,
		"SESSION_SIGNING_KEY":   demoSigningKey,
		"SESSION_COOKIE_SECURE": "false",
	})
}

// assertNoSecrets checks that no configured secret reached the output.
func assertNoSecrets(t *testing.T, output string) {
	t.Helper()
	for _, secret := range []string{auditPassword, demoPassword, demoSigningKey} {
		assert.NotContains(t, output, secret, "a secret appeared in process output")
	}
}

func TestBlackBox_MigrateSeedServe_OverRealProcessesAndHTTP(t *testing.T) {
	bin := buildCommands(t)
	db := pgtest.StartWithPassword(t, auditPassword)
	env := apiEnv(db.URL)
	var output syncBuffer

	// migrate and seed run from this package directory, two levels below the
	// repository root, so path resolution is exercised as well.
	runToCompletion(t, bin["migrate"], env, &output, "up")
	for attempt := 1; attempt <= 2; attempt++ {
		runToCompletion(t, bin["seed"], env, &output)
		assert.Equal(t, 12, pgtest.Count(t, db.Pool, "meters"), "seed attempt %d", attempt)
		assert.Equal(t, 4032, pgtest.Count(t, db.Pool, "readings"), "seed attempt %d", attempt)
		assert.Equal(t, 4, pgtest.Count(t, db.Pool, "events"), "seed attempt %d", attempt)
	}

	// Plain writers (not pipes): Wait returns only after all output was copied.
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

	base := "http://" + awaitListening(t, &output)

	assertJSON(t, base+"/healthz", http.StatusOK, "ok")
	assertJSON(t, base+"/readyz", http.StatusOK, "ready")
	resp, err := http.Get(base + "/api/v1/meters")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "the product API requires a session")
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	c := &client{t: t, base: base, http: &http.Client{Jar: jar, Timeout: 25 * time.Second}}
	code, _, _ := c.call(http.MethodPost, "/api/v1/auth/login", `{"username":"`+demoUser+`","password":"`+demoPassword+`"}`)
	require.Equal(t, http.StatusOK, code)

	// Freeze PostgreSQL (same port afterwards): the process stays alive but is not ready.
	docker(t, "pause", db.Container.GetContainerID())
	t.Cleanup(func() { _ = exec.Command("docker", "unpause", db.Container.GetContainerID()).Run() })
	eventually(t, 20*time.Second, "readyz reports 503 while PostgreSQL is paused", func() bool {
		return status(base+"/readyz") == http.StatusServiceUnavailable
	})
	assertJSON(t, base+"/healthz", http.StatusOK, "ok")
	code, header, body := c.call(http.MethodGet, "/api/v1/meters", "")
	require.Equal(t, http.StatusServiceUnavailable, code)
	publicError := body["error"].(map[string]any)
	assert.Equal(t, "request_timeout", publicError["code"])
	assert.Equal(t, header.Get("X-Request-ID"), publicError["request_id"])
	assert.NotContains(t, publicError["message"], "postgres")

	docker(t, "unpause", db.Container.GetContainerID())
	eventually(t, 30*time.Second, "readyz recovers after PostgreSQL resumes", func() bool {
		return status(base+"/readyz") == http.StatusOK
	})
	c.ok("/api/v1/meters")

	if runtime.GOOS == "windows" {
		// Windows cannot deliver SIGTERM/Ctrl+C to a child process from a test;
		// graceful shutdown is proven in-process by TestAPI_HealthReadinessAndGracefulShutdown.
		require.NoError(t, api.Process.Kill())
		<-done
		t.Log("windows: api terminated with Kill; graceful shutdown is verified by the in-process test")
	} else {
		require.NoError(t, api.Process.Signal(syscall.SIGTERM))
		select {
		case <-done:
			require.NoError(t, waitErr, "api must exit cleanly on SIGTERM")
		case <-time.After(15 * time.Second):
			t.Fatal("api did not exit within 15s of SIGTERM")
		}
		assert.Contains(t, output.String(), "shutdown complete")
	}

	assertNoSecrets(t, output.String())
	assert.Contains(t, output.String(), "xxxxx", "the database URL is logged with a masked password")
}

func buildCommands(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	bins := map[string]string{}
	for _, name := range []string{"api", "migrate", "seed"} {
		out := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			out += ".exe"
		}
		build := exec.Command("go", "build", "-o", out, "bia-energy.local/backend/cmd/"+name)
		combined, err := build.CombinedOutput()
		require.NoError(t, err, "go build cmd/%s: %s", name, combined)
		bins[name] = out
	}
	return bins
}

// processEnv inherits the environment but replaces the application variables,
// so a developer's own DATABASE_URL can never leak into the test.
func processEnv(overrides map[string]string) []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if _, replaced := overrides[strings.ToUpper(key)]; !replaced {
			env = append(env, kv)
		}
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

func runToCompletion(t *testing.T, path string, env []string, output io.Writer, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = env
	var own syncBuffer
	cmd.Stdout = io.MultiWriter(output, &own)
	cmd.Stderr = io.MultiWriter(output, &own)
	require.NoError(t, cmd.Run(), "%s %v failed:\n%s", filepath.Base(path), args, own.String())
}

// awaitListening polls the api's JSON log until it reports the bound address.
func awaitListening(t *testing.T, output *syncBuffer) string {
	t.Helper()
	var addr string
	eventually(t, 60*time.Second, "api reports its listening address", func() bool {
		for _, line := range strings.Split(output.String(), "\n") {
			var entry struct {
				Msg  string `json:"msg"`
				Addr string `json:"addr"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Msg == "api listening" {
				addr = entry.Addr
				return true
			}
		}
		return false
	})
	return addr
}

func assertJSON(t *testing.T, url string, wantCode int, wantStatus string) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	var body struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, wantCode, resp.StatusCode, url)
	assert.Equal(t, wantStatus, body.Status, url)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), url)
}

func status(url string) int {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

// eventually polls cond until it holds or the bound expires.
func eventually(t *testing.T, bound time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s: %s", bound, what)
}

func docker(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	require.NoError(t, err, "docker %v: %s", args, out)
}

type syncBuffer struct {
	mu sync.Mutex
	sb strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sb.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sb.String()
}

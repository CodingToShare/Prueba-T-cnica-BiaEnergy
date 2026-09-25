package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/auth"
)

// Test-only credentials.
const (
	testUser     = "test-operator"
	testPassword = "test-password-123"
)

var testAuth = auth.Config{
	Username:   testUser,
	Password:   testPassword,
	SigningKey: []byte("test-signing-key-0123456789abcdef-test"),
}

type pinger struct{ err error }

func (p pinger) Ping(context.Context) error { return p.err }

type fixture struct {
	handler http.Handler
	auth    *auth.Manager
	logs    *bytes.Buffer
	now     *time.Time
}

// newFixture builds the real router with no product services: every test
// here stops before a service would be called (auth, validation, routing).
func newFixture(t *testing.T, db pinger) *fixture {
	t.Helper()
	now := time.Now()
	f := &fixture{logs: &bytes.Buffer{}, now: &now}
	var err error
	f.auth, err = auth.NewManager(testAuth, func() time.Time { return *f.now })
	require.NoError(t, err)
	f.handler = NewRouter(Deps{
		Logger: slog.New(slog.NewJSONHandler(f.logs, nil)),
		DB:     db,
		Auth:   f.auth,
	})
	return f
}

func (f *fixture) do(method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func (f *fixture) sessionCookie(t *testing.T) *http.Cookie {
	t.Helper()
	s, token, err := f.auth.Issue()
	require.NoError(t, err)
	return f.auth.SessionCookie(token, s)
}

// apiErr decodes the standard error body and checks it matches the header.
func apiErr(t *testing.T, w *httptest.ResponseRecorder, status int, code string) errorDetail {
	t.Helper()
	require.Equal(t, status, w.Code, w.Body.String())
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var body errorBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "error bodies are JSON, never HTML")
	assert.Equal(t, code, body.Error.Code)
	assert.NotEmpty(t, body.Error.Message)
	assert.Equal(t, w.Header().Get(RequestIDHeader), body.Error.RequestID)
	return body.Error
}

var protectedRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/v1/auth/session"},
	{http.MethodGet, "/api/v1/meters"},
	{http.MethodGet, "/api/v1/meters/M-1"},
	{http.MethodGet, "/api/v1/meters/M-1/readings"},
	{http.MethodGet, "/api/v1/anomalies"},
	{http.MethodGet, "/api/v1/anomalies/1"},
	{http.MethodPost, "/api/v1/ai/analyze"},
	{http.MethodGet, "/api/v1/ai/analysis/1"},
	{http.MethodGet, "/api/v1/dashboard/summary"},
}

func TestHealthAndReadiness_ArePublic(t *testing.T) {
	f := newFixture(t, pinger{})
	assert.Equal(t, http.StatusOK, f.do(http.MethodGet, "/healthz", "").Code)
	assert.Equal(t, http.StatusOK, f.do(http.MethodGet, "/readyz", "").Code)

	down := newFixture(t, pinger{err: errors.New("db down")})
	assert.Equal(t, http.StatusServiceUnavailable, down.do(http.MethodGet, "/readyz", "").Code)
	assert.Equal(t, http.StatusOK, down.do(http.MethodGet, "/healthz", "").Code)
}

func TestProtectedRoutes_WithoutAValidSession_Answer401(t *testing.T) {
	f := newFixture(t, pinger{})
	valid := f.sessionCookie(t)
	tampered := *valid
	tampered.Value = valid.Value[:len(valid.Value)-2] + "xx"

	for _, rt := range protectedRoutes {
		apiErr(t, f.do(rt.method, rt.path, ""), http.StatusUnauthorized, "unauthorized")
		apiErr(t, f.do(rt.method, rt.path, "", &tampered), http.StatusUnauthorized, "unauthorized")
		apiErr(t, f.do(rt.method, rt.path, "", &http.Cookie{Name: auth.CookieName, Value: "garbage"}), http.StatusUnauthorized, "unauthorized")
	}

	*f.now = f.now.Add(auth.SessionTTL + time.Second)
	apiErr(t, f.do(http.MethodGet, "/api/v1/meters", "", valid), http.StatusUnauthorized, "unauthorized")
}

func TestLogin_SuccessSetsASecureAttributeSessionCookie(t *testing.T) {
	f := newFixture(t, pinger{})

	w := f.do(http.MethodPost, "/api/v1/auth/login", `{"username":"test-operator","password":"test-password-123"}`)

	require.Equal(t, http.StatusOK, w.Code)
	var body sessionDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Authenticated)
	assert.Equal(t, testUser, body.Username)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, auth.CookieName, c.Name)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, "/", c.Path)
	assert.NotContains(t, c.Value, testPassword)

	session := f.do(http.MethodGet, "/api/v1/auth/session", "", c)
	require.Equal(t, http.StatusOK, session.Code)
	assert.Contains(t, session.Body.String(), `"username":"test-operator"`)
	assert.NotContains(t, f.logs.String(), testPassword, "passwords are never logged")
	assert.NotContains(t, f.logs.String(), c.Value, "session cookies are never logged")
}

func TestLogin_Failures(t *testing.T) {
	f := newFixture(t, pinger{})

	wrong := apiErr(t, f.do(http.MethodPost, "/api/v1/auth/login", `{"username":"test-operator","password":"nope-nope-nope"}`),
		http.StatusUnauthorized, "invalid_credentials")
	unknown := apiErr(t, f.do(http.MethodPost, "/api/v1/auth/login", `{"username":"someone","password":"test-password-123"}`),
		http.StatusUnauthorized, "invalid_credentials")
	assert.Equal(t, wrong.Message, unknown.Message, "the message never reveals which field was wrong")
	assert.NotContains(t, wrong.Message, "nope-nope-nope")

	for _, body := range []string{``, `not json`, `{"username":"test-operator"}`, `{"username":"a","password":"b","extra":1}`, `[]`} {
		apiErr(t, f.do(http.MethodPost, "/api/v1/auth/login", body), http.StatusBadRequest, "invalid_request")
	}
	apiErr(t, f.do(http.MethodPost, "/api/v1/auth/login", `{"username":"`+strings.Repeat("a", 5000)+`","password":"x"}`),
		http.StatusBadRequest, "invalid_request")
	assert.NotContains(t, f.logs.String(), "nope-nope-nope")
}

func TestLogout_ClearsTheCookieAndIsIdempotent(t *testing.T) {
	f := newFixture(t, pinger{})
	for range 2 {
		w := f.do(http.MethodPost, "/api/v1/auth/logout", "", f.sessionCookie(t))
		require.Equal(t, http.StatusNoContent, w.Code)
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, auth.CookieName, cookies[0].Name)
		assert.Empty(t, cookies[0].Value)
		assert.Negative(t, cookies[0].MaxAge)
	}
	assert.Equal(t, http.StatusNoContent, f.do(http.MethodPost, "/api/v1/auth/logout", "").Code, "without a session too")
}

func TestRequestID_IsRandomHexWithoutHostNameAndMatchesTheErrorBody(t *testing.T) {
	f := newFixture(t, pinger{})
	host, _ := os.Hostname()

	first := apiErr(t, f.do(http.MethodGet, "/api/v1/meters", ""), http.StatusUnauthorized, "unauthorized")
	second := apiErr(t, f.do(http.MethodGet, "/api/v1/meters", ""), http.StatusUnauthorized, "unauthorized")

	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{32}$`), first.RequestID)
	assert.NotEqual(t, first.RequestID, second.RequestID)
	if host != "" {
		assert.NotContains(t, strings.ToLower(first.RequestID), strings.ToLower(host))
	}
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set(RequestIDHeader, "client-chosen-id")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	assert.NotEqual(t, "client-chosen-id", w.Header().Get(RequestIDHeader), "incoming IDs are not trusted")
}

func TestUnknownRoutesAndMethods_AnswerJSONErrors(t *testing.T) {
	f := newFixture(t, pinger{})
	apiErr(t, f.do(http.MethodGet, "/nothing-here", ""), http.StatusNotFound, "not_found")
	apiErr(t, f.do(http.MethodGet, "/api/v1/unknown", ""), http.StatusNotFound, "not_found")
	apiErr(t, f.do(http.MethodDelete, "/healthz", ""), http.StatusMethodNotAllowed, "method_not_allowed")
	apiErr(t, f.do(http.MethodGet, "/api/v1/auth/login", ""), http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestQueryValidation_RejectsInvalidInputBeforeAnyService(t *testing.T) {
	f := newFixture(t, pinger{})
	c := f.sessionCookie(t)
	tests := []struct{ path, code string }{
		{"/api/v1/meters?limit=0", "invalid_query"},
		{"/api/v1/meters?limit=101", "invalid_query"},
		{"/api/v1/meters?limit=ten", "invalid_query"},
		{"/api/v1/meters?limit=5&limit=6", "invalid_query"},
		{"/api/v1/meters?offset=-1", "invalid_query"},
		{"/api/v1/meters?sort=name", "invalid_query"},
		{"/api/v1/meters?sort=consumption;DROP%20TABLE%20meters", "invalid_query"},
		{"/api/v1/meters?order=up", "invalid_query"},
		{"/api/v1/meters?status=ok", "invalid_query"},
		{"/api/v1/meters?search=" + strings.Repeat("x", 65), "invalid_query"},
		{"/api/v1/meters/bad%20id", "invalid_meter_id"},
		{"/api/v1/meters/" + strings.Repeat("M", 65), "invalid_meter_id"},
		{"/api/v1/meters/M-1/readings?from=2026-09-12T14:00:00Z", "invalid_timestamp"},
		{"/api/v1/meters/M-1/readings?to=2026-09-12", "invalid_timestamp"},
		{"/api/v1/meters/M-1/readings?from=2026-09-13T00:00:00&to=2026-09-12T00:00:00", "invalid_range"},
		{"/api/v1/meters/M-1/readings?limit=1001", "invalid_query"},
		{"/api/v1/anomalies?severity=URGENT", "invalid_query"},
		{"/api/v1/anomalies?type=anomaly", "invalid_query"},
		{"/api/v1/anomalies?meter_id=%27%20OR%201=1", "invalid_meter_id"},
		{"/api/v1/anomalies?limit=0", "invalid_query"},
		{"/api/v1/anomalies/abc", "invalid_anomaly_id"},
		{"/api/v1/anomalies/0", "invalid_anomaly_id"},
		{"/api/v1/ai/analysis/-4", "invalid_analysis_id"},
		{"/api/v1/ai/analysis/9223372036854775808", "invalid_analysis_id"},
	}
	for _, tc := range tests {
		apiErr(t, f.do(http.MethodGet, tc.path, "", c), http.StatusBadRequest, tc.code)
	}
}

func TestRecoverer_TurnsAPanicIntoTheStandard500(t *testing.T) {
	logs := &bytes.Buffer{}
	h := withRequestID(recoverer(slog.New(slog.NewJSONHandler(logs, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret internal state")
	})))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	body := apiErr(t, w, http.StatusInternalServerError, "internal_error")
	assert.NotContains(t, body.Message, "secret internal state")
	assert.NotContains(t, logs.String(), "secret internal state", "panic values may contain secrets")
	assert.Contains(t, logs.String(), "panic while serving request")
}

func TestRequestLog_HasTheStructuredFields(t *testing.T) {
	f := newFixture(t, pinger{})
	w := f.do(http.MethodGet, "/api/v1/meters?search=M-1", "")

	var line map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(lastLine(f.logs.String()))), &line))
	assert.Equal(t, "http request", line["msg"])
	assert.Equal(t, w.Header().Get(RequestIDHeader), line["request_id"])
	assert.Equal(t, "GET", line["method"])
	assert.Equal(t, "/api/v1/meters", line["path"], "query strings are not logged")
	assert.EqualValues(t, http.StatusUnauthorized, line["status"])
	assert.Contains(t, line, "duration_ms")
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

func TestRespond_MapsUnknownErrorsWithoutLeakingThem(t *testing.T) {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	call := func(err error) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { respond(logger, w, r, err) }))
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		return w
	}

	internal := apiErr(t, call(errors.New(`pq: relation "secret_table" does not exist`)), http.StatusInternalServerError, "internal_error")
	assert.NotContains(t, internal.Message, "secret_table")
	assert.Contains(t, logs.String(), "secret_table", "the cause is logged server-side")

	apiErr(t, call(fmt.Errorf("list meters: %w", context.DeadlineExceeded)), http.StatusServiceUnavailable, "request_timeout")
	apiErr(t, call(fmt.Errorf("wrapped: %w", errInvalidCredentials)), http.StatusUnauthorized, "invalid_credentials")
}

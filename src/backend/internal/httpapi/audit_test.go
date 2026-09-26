package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogin_AuditWholeBodyAndMediaType(t *testing.T) {
	valid := `{"username":"` + testUser + `","password":"` + testPassword + `"}`
	for _, tc := range []struct {
		name, body, media string
		status            int
	}{
		{"single object", valid, "application/json", 200},
		{"charset", valid + " \n", "application/json; charset=utf-8", 200},
		{"second object", valid + " {}", "application/json", 400},
		{"trailing garbage", valid + " rubbish", "application/json", 400},
		{"oversized whitespace", valid + strings.Repeat(" ", maxLoginBody), "application/json", 400},
		{"wrong media", valid, "text/plain", 400},
		{"missing media", valid, "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, pinger{})
			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			if tc.status == 400 {
				apiErr(t, w, 400, "invalid_request")
				require.Empty(t, w.Result().Cookies())
			} else {
				require.Equal(t, tc.status, w.Code)
			}
		})
	}
}

func TestRecoverer_AuditPanicValueIsNotLogged(t *testing.T) {
	var logs bytes.Buffer
	h := withRequestID(recoverer(slog.New(slog.NewJSONHandler(&logs, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(testPassword) })))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	apiErr(t, w, 500, "internal_error")
	require.NotContains(t, logs.String(), testPassword)
}

func TestRequestID_AuditConcurrentUniquenessAndLogCorrelation(t *testing.T) {
	var logs bytes.Buffer // slog serializes writes; read only after all requests finish
	h := withRequestID(requestLogger(slog.New(slog.NewJSONHandler(&logs, nil)), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeError(w, r, errNotFound) })))
	const count = 64
	responses := make([]*httptest.ResponseRecorder, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodGet, "/missing", nil)
			r.Header.Set(RequestIDHeader, "untrusted")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			responses[i] = w
		}()
	}
	wg.Wait()
	ids := map[string]bool{}
	for _, w := range responses {
		apiErr(t, w, 404, "not_found")
		id := w.Header().Get(RequestIDHeader)
		require.Len(t, id, 32)
		require.False(t, ids[id])
		ids[id] = true
	}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		id := row["request_id"].(string)
		require.True(t, ids[id])
		delete(ids, id)
	}
	require.Empty(t, ids)
}

func TestWriteJSON_AuditEncodingFailureKeepsRequestID(t *testing.T) {
	h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]float64{"value": math.NaN()})
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	apiErr(t, w, 500, "internal_error")
}

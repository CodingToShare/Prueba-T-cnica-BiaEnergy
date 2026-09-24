package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func serve(t *testing.T, h http.HandlerFunc) (int, response, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var body response
	raw := rec.Body.String()
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("response is not JSON: %q", raw)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	return rec.Code, body, raw
}

func TestLiveness_ProcessRunning_Returns200(t *testing.T) {
	code, body, _ := serve(t, Liveness())

	if code != http.StatusOK || body.Status != "ok" {
		t.Fatalf("got %d %+v, want 200 ok", code, body)
	}
}

func TestReadiness_DatabaseReachable_Returns200(t *testing.T) {
	code, body, _ := serve(t, Readiness(fakePinger{}, discardLogger()))

	if code != http.StatusOK || body.Status != "ready" || body.Checks["postgres"] != "ok" {
		t.Fatalf("got %d %+v, want 200 ready with postgres ok", code, body)
	}
}

func TestReadiness_DatabaseUnreachable_Returns503WithoutLeakingError(t *testing.T) {
	secret := "password authentication failed for user bia"

	code, body, raw := serve(t, Readiness(fakePinger{err: errors.New(secret)}, discardLogger()))

	if code != http.StatusServiceUnavailable || body.Status != "not_ready" || body.Checks["postgres"] != "unavailable" {
		t.Fatalf("got %d %+v, want 503 not_ready", code, body)
	}
	if strings.Contains(raw, "password") {
		t.Fatalf("response leaks internal error details: %q", raw)
	}
}

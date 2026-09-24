//go:build integration

package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/config"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

func getJSON(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body
}

func TestAPI_HealthReadinessAndGracefulShutdown(t *testing.T) {
	db := pgtest.StartMigrated(t)
	cfg := config.Config{HTTPAddr: "127.0.0.1:0", DatabaseURL: db.URL}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addrCh := make(chan net.Addr, 1)
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, logger, func(a net.Addr) { addrCh <- a }) }()

	var base string
	select {
	case a := <-addrCh:
		base = "http://" + a.String()
	case err := <-done:
		t.Fatalf("api exited before listening: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("api did not start listening within 30s")
	}

	code, body := getJSON(t, base+"/healthz")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body["status"])

	code, body = getJSON(t, base+"/readyz")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ready", body["status"])

	resp, err := http.Get(base + "/api/v1/meters")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "no product API exists in Phase 01")

	// PostgreSQL goes away: the process stays alive but is no longer ready.
	require.NoError(t, db.Container.Stop(context.Background(), nil))
	code, body = getJSON(t, base+"/readyz")
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "not_ready", body["status"])
	code, _ = getJSON(t, base+"/healthz")
	assert.Equal(t, http.StatusOK, code)

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err, "graceful shutdown returns without error")
	case <-time.After(15 * time.Second):
		t.Fatal("api did not shut down within 15s")
	}
}

func TestAPI_DatabaseUnreachableAtStartup_FailsFast(t *testing.T) {
	cfg := config.Config{HTTPAddr: "127.0.0.1:0", DatabaseURL: "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := run(context.Background(), cfg, logger, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to PostgreSQL")
	assert.NotContains(t, err.Error(), "nothing", "the password must not appear in errors")
}

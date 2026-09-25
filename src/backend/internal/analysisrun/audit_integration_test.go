//go:build integration

package analysisrun

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"
	"time"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/platform/postgres/dbgen"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
	"github.com/stretchr/testify/require"
)

func TestRun_AuditQueuedRestartUsesActualEngineProvenance(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	old := newService(t, db.Pool, engine(t), Options{})
	queued, _, err := old.Request(ctx)
	require.NoError(t, err)
	cfg := analysis.DefaultConfig()
	cfg.MinRobustZ += 0.25
	current, err := analysis.New(cfg)
	require.NoError(t, err)
	restarted, err := NewService(db.Pool, current, "audit-new-engine", current.Config(), quiet, Options{})
	require.NoError(t, err)
	_, err = restarted.ProcessNext(ctx)
	require.NoError(t, err)
	row, err := dbgen.New(db.Pool).GetAnalysisRun(ctx, queued.ID)
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", row.Status)
	require.Equal(t, "audit-new-engine", row.EngineVersion)
	var stored analysis.Config
	require.NoError(t, json.Unmarshal(row.Configuration, &stored))
	require.Equal(t, current.Config(), stored)
}

func TestRun_AuditDatabaseOutageDuringAnalysisRecovers(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	good := newService(t, db.Pool, engine(t), Options{})
	a := completeRun(t, good)
	entered, timedOut := make(chan struct{}), make(chan struct{})
	s := newService(t, db.Pool, analyzerFunc(func(ctx context.Context, _ []analysis.Reading, _ []analysis.Event) (analysis.Result, error) {
		close(entered)
		<-ctx.Done()
		close(timedOut)
		return analysis.Result{}, ctx.Err()
	}), Options{RunTimeout: 1500 * time.Millisecond})
	b, _, err := s.Request(ctx)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := s.ProcessNext(ctx); done <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("analyzer did not start")
	}
	id := db.Container.GetContainerID()
	t.Cleanup(func() { _ = exec.Command("docker", "unpause", id).Run() })
	output, err := exec.Command("docker", "pause", id).CombinedOutput()
	require.NoError(t, err, string(output))
	select {
	case <-timedOut:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not time out")
	}
	output, err = exec.Command("docker", "unpause", id).CombinedOutput()
	require.NoError(t, err, string(output))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("failure recording did not finish")
	}
	run, err := s.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, StatusFailed, run.Status)
	require.Equal(t, CodeTimeout, *run.ErrorCode)
	require.Zero(t, countWhere(t, db.Pool, "anomalies", b.ID))
	require.Zero(t, countWhere(t, db.Pool, "analysis_meter_results", b.ID))
	current, err := LatestCompleted(ctx, dbgen.New(db.Pool))
	require.NoError(t, err)
	require.Equal(t, a.ID, current.ID)
	c := completeRun(t, good)
	require.Equal(t, StatusCompleted, c.Status)
}

func TestWork_AuditPollAndContinueAfterFailure(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	real := engine(t)
	calls := 0 // only the single worker goroutine accesses this
	s := newService(t, db.Pool, analyzerFunc(func(ctx context.Context, r []analysis.Reading, e []analysis.Event) (analysis.Result, error) {
		calls++
		if calls == 1 {
			return analysis.Result{}, errors.New("injected engine failure")
		}
		return real.Analyze(ctx, r, e)
	}), Options{PollInterval: 20 * time.Millisecond})
	done := make(chan struct{})
	go func() { defer close(done); s.Work(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("worker did not stop")
		}
	})
	// Queue through another service: its wake channel cannot notify this worker.
	producer := newService(t, db.Pool, real, Options{})
	for _, status := range []Status{StatusFailed, StatusCompleted} {
		run, created, err := producer.Request(ctx)
		require.NoError(t, err)
		require.True(t, created)
		require.Eventually(t, func() bool { r, err := s.Get(ctx, run.ID); return err == nil && r.Status == status }, 5*time.Second, 10*time.Millisecond)
		if status == StatusFailed {
			require.Zero(t, countWhere(t, db.Pool, "anomalies", run.ID))
		} else {
			require.Equal(t, 4, countWhere(t, db.Pool, "anomalies", run.ID))
		}
	}
}

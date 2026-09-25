//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"bia-energy.local/backend/internal/platform/postgres"
	"bia-energy.local/backend/internal/platform/postgres/dbgen"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestSnapshot_AuditConcurrentCommitKeepsOneView(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	require.NoError(t, postgres.ReadSnapshot(ctx, db.Pool, func(q *dbgen.Queries) error {
		before, err := q.ListAnalysisReadings(ctx)
		require.NoError(t, err)
		events, err := q.ListAnalysisEvents(ctx)
		require.NoError(t, err)
		// The separate connection commits between queries in the snapshot.
		_, err = db.Pool.Exec(ctx, `INSERT INTO events(meter_id,event_timestamp,event_type,description) VALUES ('M-101','2026-09-15','UNKNOWN','snapshot audit')`)
		require.NoError(t, err)
		after, err := q.ListAnalysisEvents(ctx)
		require.NoError(t, err)
		require.Equal(t, events, after)
		require.Len(t, before, 4032)
		return nil
	}))
	require.Equal(t, 5, pgtest.Count(t, db.Pool, "events"))
}

func TestMigrations_AuditDownTwoPreservesSeededData(t *testing.T) {
	db := pgtest.StartSeeded(t)
	ctx := context.Background()
	m := pgtest.NewMigrator(t, db.Pool)
	version, err := m.Down(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, version)
	require.Equal(t, 12, pgtest.Count(t, db.Pool, "meters"))
	require.Equal(t, 4032, pgtest.Count(t, db.Pool, "readings"))
	require.Equal(t, 4, pgtest.Count(t, db.Pool, "events"))
	var total float64
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT sum(consumption_kwh)::float8 FROM readings").Scan(&total))
	require.InDelta(t, 155250.85, total, 1e-6)
	versions, err := m.Up(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{2}, versions)
	pgtest.Seed(t, db.Pool)
	require.Equal(t, 4032, pgtest.Count(t, db.Pool, "readings"))
}

func TestConstraints_AuditRejectsInvalidAnalysisStates(t *testing.T) {
	db := pgtest.StartMigrated(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, status, stage string
		progress            int
		terminal            bool
		code, message       *string
		count               *int
	}{
		{name: "negative progress", status: "QUEUED", stage: "QUEUED", progress: -1},
		{name: "excess progress", status: "QUEUED", stage: "QUEUED", progress: 101},
		{name: "completed without time", status: "COMPLETED", stage: "COMPLETED", progress: 100},
		{name: "completed without results", status: "COMPLETED", stage: "COMPLETED", progress: 100, terminal: true},
		{name: "completed incomplete progress", status: "COMPLETED", stage: "COMPLETED", progress: 85, terminal: true, count: new(int)},
		{name: "failed without safe error", status: "FAILED", stage: "FAILED", progress: 35, terminal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.Pool.Exec(ctx, `INSERT INTO analysis_runs(engine_version,configuration,status,stage,progress_percent,completed_at,error_code,error_message,findings_count,high_priority_count) VALUES ('audit','{}',$1,$2,$3,CASE WHEN $4 THEN now() END,$5,$6,$7,$7)`, tc.status, tc.stage, tc.progress, tc.terminal, tc.code, tc.message, tc.count)
			var pe *pgconn.PgError
			require.ErrorAs(t, err, &pe)
			require.Equal(t, "23514", pe.Code)
		})
	}
}

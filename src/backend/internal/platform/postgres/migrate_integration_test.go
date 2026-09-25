//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

func tableNames(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name <> 'goose_db_version' ORDER BY table_name`)
	require.NoError(t, err)
	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err())
	return names
}

func TestMigrations_EmptyDatabase_UpDownUp(t *testing.T) {
	ctx := context.Background()
	db := pgtest.Start(t)
	m := pgtest.NewMigrator(t, db.Pool)

	version, err := m.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), version, "a fresh database has no schema")

	applied, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, applied)
	assert.Equal(t, allTables, tableNames(t, db.Pool))
	assert.True(t, hasColumn(t, db.Pool, "anomalies", "explanation"))
	assert.True(t, hasColumn(t, db.Pool, "analysis_runs", "explanation_configuration"))

	again, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Empty(t, again, "re-running up is a no-op")

	// A run caught in the explanation stage does not block the rollback.
	_, err = db.Pool.Exec(ctx, `INSERT INTO analysis_runs (engine_version, configuration, status, stage, progress_percent)
		VALUES ('v', '{}', 'RUNNING', 'GENERATING_EXPLANATIONS', 50)`)
	require.NoError(t, err)
	rolledBack, err := m.Down(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), rolledBack)
	assert.Equal(t, allTables, tableNames(t, db.Pool), "down 3 removes only the explanation columns")
	assert.False(t, hasColumn(t, db.Pool, "anomalies", "explanation"))
	assert.False(t, hasColumn(t, db.Pool, "analysis_runs", "explanation_configuration"))
	var stage string
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT stage FROM analysis_runs").Scan(&stage))
	assert.Equal(t, "ANALYZING", stage)
	_, err = db.Pool.Exec(ctx, "DELETE FROM analysis_runs")
	require.NoError(t, err)

	rolledBack, err = m.Down(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rolledBack)
	assert.Equal(t, sourceTables, tableNames(t, db.Pool), "down removes only the analysis tables; source data stays")

	rolledBack, err = m.Down(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rolledBack)
	assert.Empty(t, tableNames(t, db.Pool), "down removes every Phase 01 table")

	reapplied, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, reapplied)
	assert.Equal(t, allTables, tableNames(t, db.Pool))
}

func hasColumn(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2)`, table, column).Scan(&exists))
	return exists
}

var (
	sourceTables = []string{"events", "meters", "readings"}
	allTables    = []string{"analysis_meter_results", "analysis_runs", "anomalies", "events", "meters", "readings"}
)

func TestMigrations_SchemaMatchesSourceContract(t *testing.T) {
	db := pgtest.StartMigrated(t)

	rows, err := db.Pool.Query(context.Background(),
		`SELECT table_name || '.' || column_name, data_type, is_nullable
		 FROM information_schema.columns WHERE table_schema = 'public' AND table_name IN ('meters','readings','events')`)
	require.NoError(t, err)
	columns := map[string]string{}
	for rows.Next() {
		var name, dataType, nullable string
		require.NoError(t, rows.Scan(&name, &dataType, &nullable))
		columns[name] = dataType + " " + nullable
	}
	require.NoError(t, rows.Err())

	assert.Equal(t, map[string]string{
		"meters.meter_id":            "text NO",
		"readings.meter_id":          "text NO",
		"readings.reading_timestamp": "timestamp without time zone NO",
		"readings.consumption_kwh":   "numeric NO",
		"readings.voltage_v":         "numeric NO",
		"readings.current_a":         "numeric NO",
		"readings.power_factor":      "numeric NO",
		"readings.source_status":     "text NO",
		"events.id":                  "bigint NO",
		"events.meter_id":            "text NO",
		"events.event_timestamp":     "timestamp without time zone NO",
		"events.event_type":          "text NO",
		"events.description":         "text NO",
	}, columns)
}

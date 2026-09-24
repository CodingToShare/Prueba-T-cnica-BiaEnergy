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
	assert.Equal(t, []int64{1}, applied)
	assert.Equal(t, []string{"events", "meters", "readings"}, tableNames(t, db.Pool))

	again, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Empty(t, again, "re-running up is a no-op")

	rolledBack, err := m.Down(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rolledBack)
	assert.Empty(t, tableNames(t, db.Pool), "down removes every Phase 01 table")

	reapplied, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, reapplied)
	assert.Equal(t, []string{"events", "meters", "readings"}, tableNames(t, db.Pool))
}

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

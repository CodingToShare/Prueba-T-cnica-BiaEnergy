//go:build integration

package ingestion_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/ingestion"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

func fixtureDataset(t *testing.T) ingestion.Dataset {
	t.Helper()
	d, err := ingestion.ParseDir("testdata/valid")
	require.NoError(t, err)
	return d
}

func counts(t *testing.T, pool *pgxpool.Pool) [3]int {
	t.Helper()
	return [3]int{pgtest.Count(t, pool, "meters"), pgtest.Count(t, pool, "readings"), pgtest.Count(t, pool, "events")}
}

func TestLoad_Fixtures_InsertsThenIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := pgtest.StartMigrated(t)
	d := fixtureDataset(t)

	first, err := ingestion.Load(ctx, db.Pool, d)
	require.NoError(t, err)
	assert.Equal(t, ingestion.Counts{Inserted: 3}, first.Meters)
	assert.Equal(t, ingestion.Counts{Inserted: 6}, first.Readings)
	assert.Equal(t, ingestion.Counts{Inserted: 2}, first.Events)
	assert.Equal(t, [3]int{3, 6, 2}, counts(t, db.Pool))

	second, err := ingestion.Load(ctx, db.Pool, d)
	require.NoError(t, err)
	assert.Equal(t, ingestion.Counts{Unchanged: 3}, second.Meters)
	assert.Equal(t, ingestion.Counts{Unchanged: 6}, second.Readings)
	assert.Equal(t, ingestion.Counts{Unchanged: 2}, second.Events)
	assert.Equal(t, [3]int{3, 6, 2}, counts(t, db.Pool), "re-import must not duplicate rows")
}

func TestLoad_ChangedSourceValue_UpdatesOnlyThatReading(t *testing.T) {
	ctx := context.Background()
	db := pgtest.StartMigrated(t)
	d := fixtureDataset(t)
	_, err := ingestion.Load(ctx, db.Pool, d)
	require.NoError(t, err)

	d.Readings[0].VoltageV = 199.5
	result, err := ingestion.Load(ctx, db.Pool, d)

	require.NoError(t, err)
	assert.Equal(t, ingestion.Counts{Updated: 1, Unchanged: 5}, result.Readings)
	var voltage string
	require.NoError(t, db.Pool.QueryRow(ctx,
		`SELECT voltage_v::text FROM readings WHERE meter_id = 'TEST-A' AND reading_timestamp = '2026-01-05 00:00:00'`).Scan(&voltage))
	assert.Equal(t, "199.5", voltage)
}

func TestLoad_PersistsSourceValuesAndWallClockExactly(t *testing.T) {
	ctx := context.Background()
	db := pgtest.StartMigrated(t)
	_, err := ingestion.Load(ctx, db.Pool, fixtureDataset(t))
	require.NoError(t, err)

	var ts, consumption, current, pf, status string
	require.NoError(t, db.Pool.QueryRow(ctx, `
		SELECT reading_timestamp::text, consumption_kwh::text, current_a::text, power_factor::text, source_status
		FROM readings WHERE meter_id = 'TEST-A' AND reading_timestamp = '2026-01-05 02:00:00'`).
		Scan(&ts, &consumption, &current, &pf, &status))
	assert.Equal(t, []string{"2026-01-05 02:00:00", "10.75", "51.5", "0.951", "OK"}, []string{ts, consumption, current, pf, status})

	var eventTS string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT event_timestamp::text FROM events WHERE meter_id = 'TEST-A'`).Scan(&eventTS))
	assert.Equal(t, "2026-01-05 01:00:00", eventTS)
}

func TestLoad_FailureMidway_WritesNothing(t *testing.T) {
	ctx := context.Background()
	db := pgtest.StartMigrated(t)
	d := fixtureDataset(t)
	// Bypasses the parser on purpose: the database check must still reject it.
	d.Readings[len(d.Readings)-1].SourceStatus = "   "

	_, err := ingestion.Load(ctx, db.Pool, d)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading for meter TEST-B at 2026-01-05 02:00:00", "the error names the failing row")
	assert.Equal(t, [3]int{0, 0, 0}, counts(t, db.Pool), "a failed import must leave no partial data")
}

func TestSchema_EnforcesIntegrityInTheDatabase(t *testing.T) {
	ctx := context.Background()
	db := pgtest.StartMigrated(t)
	_, err := ingestion.Load(ctx, db.Pool, fixtureDataset(t))
	require.NoError(t, err)

	tests := map[string]struct {
		sql      string
		wantCode string
	}{
		"duplicate reading key": {
			`INSERT INTO readings VALUES ('TEST-A', '2026-01-05 00:00:00', 1, 1, 1, 1, 'OK')`, "23505"},
		"reading for unknown meter": {
			`INSERT INTO readings VALUES ('NOPE', '2026-01-05 00:00:00', 1, 1, 1, 1, 'OK')`, "23503"},
		"non-finite measurement": {
			`INSERT INTO readings VALUES ('TEST-A', '2026-02-01 00:00:00', 'NaN', 1, 1, 1, 'OK')`, "23514"},
		"blank meter id": {
			`INSERT INTO meters VALUES ('  ')`, "23514"},
		"duplicate event": {
			`INSERT INTO events (meter_id, event_timestamp, event_type, description)
			 VALUES ('TEST-A', '2026-01-05 01:00', 'OPERATIONAL_CHANGE', 'Synthetic test event')`, "23505"},
		"event for unknown meter": {
			`INSERT INTO events (meter_id, event_timestamp, event_type, description)
			 VALUES ('NOPE', '2026-01-05 01:00', 'UNKNOWN', 'x')`, "23503"},
		"blank event type": {
			`INSERT INTO events (meter_id, event_timestamp, event_type, description)
			 VALUES ('TEST-A', '2026-01-06 01:00', ' ', 'x')`, "23514"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := db.Pool.Exec(ctx, tc.sql)

			var pgErr *pgconn.PgError
			require.True(t, errors.As(err, &pgErr), "expected a PostgreSQL error, got %v", err)
			assert.Equal(t, tc.wantCode, pgErr.Code)
		})
	}

	// Unusual physical values stay storable: they are analytical signals.
	_, err = db.Pool.Exec(ctx, `INSERT INTO readings VALUES ('TEST-A', $1, -5, 0, 999999.999, 1.8, 'OK')`,
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)

	// New event types need no schema change (no enum).
	_, err = db.Pool.Exec(ctx, `INSERT INTO events (meter_id, event_timestamp, event_type, description)
		VALUES ('TEST-B', '2026-01-07 00:00', 'FUTURE_EVENT_TYPE', 'x')`)
	assert.NoError(t, err)
}

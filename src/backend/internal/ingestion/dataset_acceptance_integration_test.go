//go:build integration

package ingestion_test

// Data-integrity acceptance for the supplied challenge dataset in data/input.
// The expected figures are the dataset contract (12 meters, 14 days of hourly
// readings, 4 events); they are not analytics behaviour.

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/ingestion"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
	"bia-energy.local/backend/internal/workspace"
)

var expectedMeterIDs = []string{
	"M-101", "M-102", "M-103", "M-104", "M-105", "M-106",
	"M-107", "M-108", "M-109", "M-110", "M-111", "M-112",
}

func sourceDir(t *testing.T) string {
	t.Helper()
	dir, err := workspace.FindDir("data/input")
	require.NoError(t, err)
	for _, name := range []string{ingestion.ReadingsFile, ingestion.EventsFile} {
		_, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err, "the supplied %s must be present in data/input", name)
	}
	return dir
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestAcceptance_SuppliedDataset_LoadsCompletelyAndIdempotently(t *testing.T) {
	ctx := context.Background()
	dir := sourceDir(t)
	hashesBefore := map[string]string{}
	for _, f := range []string{ingestion.ReadingsFile, ingestion.EventsFile} {
		hashesBefore[f] = fileHash(t, filepath.Join(dir, f))
	}

	dataset, err := ingestion.ParseDir(dir)
	require.NoError(t, err, "the supplied dataset must pass validation")
	report := ingestion.BuildReport(dataset)
	assert.Equal(t, 0, report.HourlyGaps, "hourly series must be continuous")
	assert.Equal(t, 0, report.IrregularSteps)

	db := pgtest.StartMigrated(t)

	first, err := ingestion.Load(ctx, db.Pool, dataset)
	require.NoError(t, err)
	assert.Equal(t, ingestion.Counts{Inserted: 12}, first.Meters)
	assert.Equal(t, ingestion.Counts{Inserted: 4032}, first.Readings)
	assert.Equal(t, ingestion.Counts{Inserted: 4}, first.Events)
	assertDatasetContract(t, db)
	versionsBefore := rowVersions(t, db)

	second, err := ingestion.Load(ctx, db.Pool, dataset)
	require.NoError(t, err)
	assert.Equal(t, ingestion.Counts{Unchanged: 12}, second.Meters)
	assert.Equal(t, ingestion.Counts{Unchanged: 4032}, second.Readings)
	assert.Equal(t, ingestion.Counts{Unchanged: 4}, second.Events)
	assertDatasetContract(t, db)
	assert.Equal(t, versionsBefore, rowVersions(t, db),
		"re-importing identical data must not rewrite any row (ctid/xmin unchanged)")

	assertValuesMatchSource(t, db, filepath.Join(dir, ingestion.ReadingsFile))
	assertEventsMatchSource(t, db, filepath.Join(dir, ingestion.EventsFile))

	for f, before := range hashesBefore {
		assert.Equal(t, before, fileHash(t, filepath.Join(dir, f)), "%s must not be modified", f)
	}
}

func assertDatasetContract(t *testing.T, db *pgtest.DB) {
	t.Helper()
	ctx := context.Background()

	assert.Equal(t, 12, pgtest.Count(t, db.Pool, "meters"))
	assert.Equal(t, 4032, pgtest.Count(t, db.Pool, "readings"))
	assert.Equal(t, 4, pgtest.Count(t, db.Pool, "events"))

	rows, err := db.Pool.Query(ctx, `SELECT meter_id, count(*) FROM readings GROUP BY meter_id ORDER BY meter_id`)
	require.NoError(t, err)
	perMeter := map[string]int{}
	var ids []string
	for rows.Next() {
		var id string
		var n int
		require.NoError(t, rows.Scan(&id, &n))
		perMeter[id] = n
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, expectedMeterIDs, ids)
	for _, id := range expectedMeterIDs {
		assert.Equal(t, 336, perMeter[id], "readings for %s", id)
	}

	var minTS, maxTS string
	var distinctHours int
	require.NoError(t, db.Pool.QueryRow(ctx, `
		SELECT min(reading_timestamp)::text, max(reading_timestamp)::text, count(DISTINCT reading_timestamp)
		FROM readings`).Scan(&minTS, &maxTS, &distinctHours))
	assert.Equal(t, "2026-09-01 00:00:00", minTS)
	assert.Equal(t, "2026-09-14 23:00:00", maxTS)
	assert.Equal(t, 336, distinctHours, "14 days x 24 hours")

	var eventMeters int
	require.NoError(t, db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM events e JOIN meters m USING (meter_id)`).Scan(&eventMeters))
	assert.Equal(t, 4, eventMeters, "every event references a known meter")
}

// assertValuesMatchSource compares every stored measurement with the decimal
// literal in the source file, proving values were persisted exactly.
func assertValuesMatchSource(t *testing.T, db *pgtest.DB, path string) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)

	source := make(map[string][]string, len(records)-1)
	for _, r := range records[1:] {
		source[r[0]+"|"+r[1]] = []string{r[2], r[3], r[4], r[5], r[6]}
	}

	rows, err := db.Pool.Query(context.Background(), `
		SELECT meter_id, to_char(reading_timestamp, 'YYYY-MM-DD HH24:MI:SS'),
		       consumption_kwh::text, voltage_v::text, current_a::text, power_factor::text, source_status
		FROM readings`)
	require.NoError(t, err)
	compared := 0
	for rows.Next() {
		var id, ts string
		stored := make([]string, 5)
		require.NoError(t, rows.Scan(&id, &ts, &stored[0], &stored[1], &stored[2], &stored[3], &stored[4]))
		want, ok := source[id+"|"+ts]
		require.True(t, ok, "stored reading %s %s is not in the source file", id, ts)
		for i := 0; i < 4; i++ {
			require.True(t, sameDecimal(want[i], stored[i]), "%s %s column %d: source %s, stored %s", id, ts, i, want[i], stored[i])
		}
		require.Equal(t, want[4], stored[4], "%s %s source_status", id, ts)
		compared++
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, 4032, compared)
}

// assertEventsMatchSource compares every stored event field with the source file.
func assertEventsMatchSource(t *testing.T, db *pgtest.DB, path string) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)

	var want [][]string
	for _, r := range records[1:] {
		want = append(want, []string{r[0], r[1], r[2], r[3]})
	}

	rows, err := db.Pool.Query(context.Background(), `
		SELECT meter_id, to_char(event_timestamp, 'YYYY-MM-DD HH24:MI'), event_type, description
		FROM events ORDER BY meter_id, event_timestamp`)
	require.NoError(t, err)
	var got [][]string
	for rows.Next() {
		row := make([]string, 4)
		require.NoError(t, rows.Scan(&row[0], &row[1], &row[2], &row[3]))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	assert.ElementsMatch(t, want, got, "stored events must equal the source rows field by field")
}

// rowVersions captures the physical location and writing transaction of
// every row; any rewrite (UPDATE, delete+insert) changes them.
func rowVersions(t *testing.T, db *pgtest.DB) []string {
	t.Helper()
	rows, err := db.Pool.Query(context.Background(), `
		SELECT 'm:' || meter_id || '|' || ctid::text || '|' || xmin::text FROM meters
		UNION ALL SELECT 'r:' || meter_id || reading_timestamp::text || '|' || ctid::text || '|' || xmin::text FROM readings
		UNION ALL SELECT 'e:' || id::text || '|' || ctid::text || '|' || xmin::text FROM events
		ORDER BY 1`)
	require.NoError(t, err)
	var out []string
	for rows.Next() {
		var v string
		require.NoError(t, rows.Scan(&v))
		out = append(out, v)
	}
	require.NoError(t, rows.Err())
	require.Len(t, out, 12+4032+4)
	return out
}

func sameDecimal(a, b string) bool {
	x, okA := new(big.Rat).SetString(a)
	y, okB := new(big.Rat).SetString(b)
	return okA && okB && x.Cmp(y) == 0
}

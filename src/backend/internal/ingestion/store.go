package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Idempotent upserts keyed by the natural keys enforced in the schema.
// RETURNING yields a row only when something was written, which lets the
// loader distinguish inserted, updated and unchanged records.
const (
	upsertMeterSQL = `
INSERT INTO meters (meter_id) VALUES ($1)
ON CONFLICT (meter_id) DO NOTHING
RETURNING true`

	upsertReadingSQL = `
INSERT INTO readings (meter_id, reading_timestamp, consumption_kwh, voltage_v, current_a, power_factor, source_status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (meter_id, reading_timestamp) DO UPDATE SET
    consumption_kwh = EXCLUDED.consumption_kwh,
    voltage_v       = EXCLUDED.voltage_v,
    current_a       = EXCLUDED.current_a,
    power_factor    = EXCLUDED.power_factor,
    source_status   = EXCLUDED.source_status
WHERE (readings.consumption_kwh, readings.voltage_v, readings.current_a, readings.power_factor, readings.source_status)
      IS DISTINCT FROM
      (EXCLUDED.consumption_kwh, EXCLUDED.voltage_v, EXCLUDED.current_a, EXCLUDED.power_factor, EXCLUDED.source_status)
RETURNING (xmax = 0)`

	upsertEventSQL = `
INSERT INTO events (meter_id, event_timestamp, event_type, description)
VALUES ($1, $2, $3, $4)
ON CONFLICT ON CONSTRAINT events_natural_key DO NOTHING
RETURNING true`
)

// Counts reports what a load did to one table.
type Counts struct {
	Inserted  int
	Updated   int
	Unchanged int
}

// LoadResult reports what a load did to each table.
type LoadResult struct {
	Meters   Counts
	Readings Counts
	Events   Counts
}

// Load writes a validated dataset in a single transaction: either every row
// is applied or none is. Re-running it with the same data changes nothing.
func Load(ctx context.Context, pool *pgxpool.Pool, d Dataset) (LoadResult, error) {
	var result LoadResult

	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("begin ingestion transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	// A batch is pipelined in one round trip; queue order is preserved, so
	// meters are written before the readings and events that reference them.
	batch := &pgx.Batch{}
	for _, id := range d.MeterIDs() {
		batch.Queue(upsertMeterSQL, id).QueryRow(countWrite(&result.Meters, "meter "+id))
	}
	for _, r := range d.Readings {
		batch.Queue(upsertReadingSQL, r.MeterID, r.Timestamp, r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor, r.SourceStatus).
			QueryRow(countUpsert(&result.Readings, "reading for meter "+r.MeterID+" at "+r.Timestamp.Format(time.DateTime)))
	}
	for _, e := range d.Events {
		batch.Queue(upsertEventSQL, e.MeterID, e.Timestamp, e.Type, e.Description).
			QueryRow(countWrite(&result.Events, "event for meter "+e.MeterID))
	}

	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return LoadResult{}, fmt.Errorf("write source data: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LoadResult{}, fmt.Errorf("commit ingestion transaction: %w", err)
	}
	return result, nil
}

// countWrite counts insert-or-nothing statements.
func countWrite(c *Counts, what string) func(pgx.Row) error {
	return func(row pgx.Row) error {
		var written bool
		switch err := row.Scan(&written); {
		case errors.Is(err, pgx.ErrNoRows):
			c.Unchanged++
		case err != nil:
			return fmt.Errorf("%s: %w", what, err)
		default:
			c.Inserted++
		}
		return nil
	}
}

// countUpsert counts statements that may insert, update or leave a row unchanged.
func countUpsert(c *Counts, what string) func(pgx.Row) error {
	return func(row pgx.Row) error {
		var inserted bool
		switch err := row.Scan(&inserted); {
		case errors.Is(err, pgx.ErrNoRows):
			c.Unchanged++
		case err != nil:
			return fmt.Errorf("%s: %w", what, err)
		case inserted:
			c.Inserted++
		default:
			c.Updated++
		}
		return nil
	}
}

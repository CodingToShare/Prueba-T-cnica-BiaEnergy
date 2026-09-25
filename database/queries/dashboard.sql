-- Dashboard summary (internal/dashboard).

-- name: GetSourceTotals :one
SELECT (SELECT count(*) FROM meters)                                    AS meters_count,
       (SELECT count(*) FROM readings)                                  AS readings_count,
       (SELECT coalesce(sum(consumption_kwh), 0) FROM readings)::float8 AS total_consumption_kwh;

-- name: GetReadingPeriod :one
-- First and last source timestamp, for all meters or one. Returns no row
-- when there are no readings.
SELECT min(reading_timestamp)::timestamp AS first_reading_at,
       max(reading_timestamp)::timestamp AS last_reading_at
FROM readings
WHERE sqlc.narg('meter_id')::text IS NULL OR meter_id = sqlc.narg('meter_id')::text
HAVING count(*) > 0;

-- name: GetLatestCompletedRunSummary :one
SELECT r.id, r.completed_at, r.findings_count, r.high_priority_count, r.aggregate_confidence
FROM analysis_runs r
WHERE r.status = 'COMPLETED'
ORDER BY r.completed_at DESC, r.id DESC
LIMIT 1;

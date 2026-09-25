-- Meter list, detail and readings (internal/meter). "Current" analytical
-- state always comes from the latest COMPLETED analysis run.

-- name: ListMeters :many
-- One query for the whole page (no per-meter queries). Sort keys are fixed
-- whitelisted values chosen by the caller; user text never reaches SQL
-- structure. Variation sorts by magnitude; meters without a value sort last.
WITH current_run AS (
    SELECT id FROM analysis_runs
    WHERE status = 'COMPLETED'
    ORDER BY completed_at DESC, id DESC
    LIMIT 1
),
totals AS (
    SELECT meter_id, sum(consumption_kwh)::float8 AS total_consumption_kwh
    FROM readings
    GROUP BY meter_id
),
top_finding AS (
    SELECT DISTINCT ON (a.meter_id)
           a.meter_id, a.id, a.type, a.severity, a.confidence, a.priority, a.consumption_deviation_pct
    FROM anomalies a
    JOIN current_run c ON a.analysis_run_id = c.id
    ORDER BY a.meter_id, a.priority
),
page AS (
    SELECT m.meter_id,
           coalesce(t.total_consumption_kwh, 0)::float8 AS total_consumption_kwh,
           mr.computed_status,
           f.id                        AS anomaly_id,
           f.type                      AS anomaly_type,
           f.severity                  AS severity,
           f.confidence                AS confidence,
           f.priority                  AS priority,
           f.consumption_deviation_pct AS variation_pct,
           CASE f.severity WHEN 'HIGH' THEN 3 WHEN 'MEDIUM' THEN 2 WHEN 'LOW' THEN 1 ELSE 0 END AS severity_rank
    FROM meters m
    LEFT JOIN totals t ON t.meter_id = m.meter_id
    LEFT JOIN current_run c ON true
    LEFT JOIN analysis_meter_results mr ON mr.analysis_run_id = c.id AND mr.meter_id = m.meter_id
    LEFT JOIN top_finding f ON f.meter_id = m.meter_id
    WHERE (sqlc.narg('search')::text IS NULL OR strpos(lower(m.meter_id), lower(sqlc.narg('search')::text)) > 0)
      AND (sqlc.narg('computed_status')::text IS NULL OR mr.computed_status = sqlc.narg('computed_status')::text)
)
SELECT meter_id, total_consumption_kwh, computed_status, anomaly_id, anomaly_type,
       severity, confidence, priority, variation_pct
FROM page
ORDER BY
    CASE WHEN @sort_key::text = 'consumption' AND @sort_desc::bool = false THEN total_consumption_kwh END ASC,
    CASE WHEN @sort_key::text = 'consumption' AND @sort_desc::bool = true THEN total_consumption_kwh END DESC,
    CASE WHEN @sort_key::text = 'variation' AND @sort_desc::bool = false THEN abs(variation_pct) END ASC NULLS LAST,
    CASE WHEN @sort_key::text = 'variation' AND @sort_desc::bool = true THEN abs(variation_pct) END DESC NULLS LAST,
    CASE WHEN @sort_key::text = 'severity' AND @sort_desc::bool = false THEN severity_rank END ASC,
    CASE WHEN @sort_key::text = 'severity' AND @sort_desc::bool = true THEN severity_rank END DESC,
    CASE WHEN @sort_key::text = 'severity' THEN priority END ASC NULLS LAST,
    CASE WHEN @sort_key::text = 'meter_id' AND @sort_desc::bool = true THEN meter_id END DESC,
    meter_id ASC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountMeters :one
WITH current_run AS (
    SELECT id FROM analysis_runs
    WHERE status = 'COMPLETED'
    ORDER BY completed_at DESC, id DESC
    LIMIT 1
)
SELECT count(*)
FROM meters m
LEFT JOIN current_run c ON true
LEFT JOIN analysis_meter_results mr ON mr.analysis_run_id = c.id AND mr.meter_id = m.meter_id
WHERE (sqlc.narg('search')::text IS NULL OR strpos(lower(m.meter_id), lower(sqlc.narg('search')::text)) > 0)
  AND (sqlc.narg('computed_status')::text IS NULL OR mr.computed_status = sqlc.narg('computed_status')::text);

-- name: GetMeterOverview :one
WITH current_run AS (
    SELECT id, completed_at FROM analysis_runs
    WHERE status = 'COMPLETED'
    ORDER BY completed_at DESC, id DESC
    LIMIT 1
),
totals AS (
    SELECT coalesce(sum(consumption_kwh), 0)::float8 AS total_consumption_kwh,
           count(*)                                  AS readings_count
    FROM readings
    WHERE meter_id = @meter_id
)
SELECT m.meter_id,
       t.total_consumption_kwh,
       t.readings_count,
       c.id           AS analysis_run_id,
       c.completed_at AS analysis_completed_at,
       mr.computed_status
FROM meters m
CROSS JOIN totals t
LEFT JOIN current_run c ON true
LEFT JOIN analysis_meter_results mr ON mr.analysis_run_id = c.id AND mr.meter_id = m.meter_id
WHERE m.meter_id = @meter_id;

-- name: ListCurrentMeterFindings :many
SELECT a.id, a.type, a.severity, a.confidence, a.priority, a.consumption_deviation_pct,
       a.started_at, a.last_observed_at, a.recommended_action, a.reason
FROM anomalies a
WHERE a.meter_id = @meter_id
  AND a.analysis_run_id = (
      SELECT id FROM analysis_runs
      WHERE status = 'COMPLETED'
      ORDER BY completed_at DESC, id DESC
      LIMIT 1
  )
ORDER BY a.priority;

-- name: MeterExists :one
SELECT EXISTS (SELECT 1 FROM meters WHERE meter_id = @meter_id);

-- name: ListMeterReadings :many
-- Half-open range [from_ts, to_ts) on source wall-clock time (ADR-008).
SELECT reading_timestamp,
       consumption_kwh::float8 AS consumption_kwh,
       voltage_v::float8       AS voltage_v,
       current_a::float8       AS current_a,
       power_factor::float8    AS power_factor,
       source_status
FROM readings
WHERE meter_id = @meter_id
  AND (sqlc.narg('from_ts')::timestamp IS NULL OR reading_timestamp >= sqlc.narg('from_ts')::timestamp)
  AND (sqlc.narg('to_ts')::timestamp IS NULL OR reading_timestamp < sqlc.narg('to_ts')::timestamp)
ORDER BY reading_timestamp
LIMIT @row_limit;

-- Anomaly list and detail (internal/anomaly). The list shows the findings of
-- the latest COMPLETED run only; findings of a run become visible only when
-- its completion transaction commits.

-- name: ListCurrentAnomalies :many
WITH current_run AS (
    SELECT id FROM analysis_runs
    WHERE status = 'COMPLETED'
    ORDER BY completed_at DESC, id DESC
    LIMIT 1
)
SELECT a.id, a.analysis_run_id, a.meter_id, a.priority, a.type, a.severity, a.confidence,
       a.status, a.started_at, a.last_observed_at, a.duration_seconds,
       a.consumption_deviation_pct, a.reason, a.recommended_action
FROM anomalies a
JOIN current_run c ON a.analysis_run_id = c.id
WHERE (sqlc.narg('meter_id')::text IS NULL OR a.meter_id = sqlc.narg('meter_id')::text)
  AND (sqlc.narg('type')::text IS NULL OR a.type = sqlc.narg('type')::text)
  AND (sqlc.narg('severity')::text IS NULL OR a.severity = sqlc.narg('severity')::text)
ORDER BY a.priority, a.id
LIMIT @row_limit OFFSET @row_offset;

-- name: CountCurrentAnomalies :one
WITH current_run AS (
    SELECT id FROM analysis_runs
    WHERE status = 'COMPLETED'
    ORDER BY completed_at DESC, id DESC
    LIMIT 1
)
SELECT count(*)
FROM anomalies a
JOIN current_run c ON a.analysis_run_id = c.id
WHERE (sqlc.narg('meter_id')::text IS NULL OR a.meter_id = sqlc.narg('meter_id')::text)
  AND (sqlc.narg('type')::text IS NULL OR a.type = sqlc.narg('type')::text)
  AND (sqlc.narg('severity')::text IS NULL OR a.severity = sqlc.narg('severity')::text);

-- name: GetAnomaly :one
SELECT id, analysis_run_id, meter_id, priority, type, severity, confidence, rule,
       recommended_action, reason, started_at, last_observed_at, duration_seconds,
       consumption_deviation_pct, status, evidence, created_at
FROM anomalies
WHERE id = @id;

-- Analysis run lifecycle and result persistence (internal/analysisrun).

-- name: CreateAnalysisRun :one
-- Inserts a QUEUED run unless another run is active; the partial unique
-- index analysis_runs_single_active then makes this return no row.
INSERT INTO analysis_runs (engine_version, configuration, explanation_configuration)
VALUES (@engine_version, @configuration, @explanation_configuration)
ON CONFLICT DO NOTHING
RETURNING id, status, stage, progress_percent, engine_version, configuration,
          meters_count, readings_count, events_count, findings_count, high_priority_count, aggregate_confidence,
          error_code, error_message, created_at, started_at, completed_at, explanation_configuration;

-- name: GetActiveAnalysisRun :one
SELECT id, status, stage, progress_percent, engine_version, configuration,
       meters_count, readings_count, events_count, findings_count, high_priority_count, aggregate_confidence,
       error_code, error_message, created_at, started_at, completed_at, explanation_configuration
FROM analysis_runs
WHERE status IN ('QUEUED', 'RUNNING');

-- name: GetAnalysisRun :one
SELECT id, status, stage, progress_percent, engine_version, configuration,
       meters_count, readings_count, events_count, findings_count, high_priority_count, aggregate_confidence,
       error_code, error_message, created_at, started_at, completed_at, explanation_configuration
FROM analysis_runs
WHERE id = @id;

-- name: ClaimQueuedAnalysisRun :one
-- Atomically moves the oldest QUEUED run to RUNNING. SKIP LOCKED lets
-- competing workers pass over a run another worker is claiming.
UPDATE analysis_runs
SET status = 'RUNNING', stage = 'LOADING_DATA', progress_percent = @progress_percent, started_at = now(),
    engine_version = @engine_version, configuration = @configuration,
    explanation_configuration = @explanation_configuration
WHERE id = (
    SELECT q.id FROM analysis_runs q
    WHERE q.status = 'QUEUED'
    ORDER BY q.id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, status, stage, progress_percent, engine_version, configuration,
          meters_count, readings_count, events_count, findings_count, high_priority_count, aggregate_confidence,
          error_code, error_message, created_at, started_at, completed_at, explanation_configuration;

-- name: SetAnalysisRunStage :execrows
UPDATE analysis_runs
SET stage = @stage, progress_percent = @progress_percent
WHERE id = @id AND status = 'RUNNING';

-- name: SetAnalysisRunSourceCounts :execrows
UPDATE analysis_runs
SET meters_count = @meters_count, readings_count = @readings_count, events_count = @events_count
WHERE id = @id AND status = 'RUNNING';

-- name: CompleteAnalysisRun :execrows
UPDATE analysis_runs
SET status = 'COMPLETED', stage = 'COMPLETED', progress_percent = 100,
    findings_count = @findings_count, high_priority_count = @high_priority_count,
    aggregate_confidence = sqlc.narg('aggregate_confidence'),
    completed_at = now()
WHERE id = @id AND status = 'RUNNING';

-- name: FailAnalysisRun :execrows
UPDATE analysis_runs
SET status = 'FAILED', stage = 'FAILED', error_code = @error_code, error_message = @error_message,
    completed_at = now()
WHERE id = @id AND status IN ('QUEUED', 'RUNNING');

-- name: FailInterruptedAnalysisRuns :many
-- Startup recovery: a RUNNING run cannot still be running after a restart.
UPDATE analysis_runs
SET status = 'FAILED', stage = 'FAILED', error_code = @error_code, error_message = @error_message,
    completed_at = now()
WHERE status = 'RUNNING'
RETURNING id;

-- name: InsertAnomaly :exec
INSERT INTO anomalies (
    analysis_run_id, meter_id, priority, type, severity, confidence, rule,
    recommended_action, reason, started_at, last_observed_at, duration_seconds,
    consumption_deviation_pct, evidence,
    explanation, explanation_source, explanation_model, explanation_prompt_version,
    explanation_generated_at, explanation_fallback_used, explanation_fallback_code
) VALUES (
    @analysis_run_id, @meter_id, @priority, @type, @severity, @confidence, @rule,
    @recommended_action, @reason, @started_at, @last_observed_at, @duration_seconds,
    @consumption_deviation_pct, @evidence,
    sqlc.narg('explanation'), sqlc.narg('explanation_source'), sqlc.narg('explanation_model'),
    sqlc.narg('explanation_prompt_version'), sqlc.narg('explanation_generated_at'),
    sqlc.narg('explanation_fallback_used'), sqlc.narg('explanation_fallback_code')
);

-- name: InsertAnalysisMeterResult :exec
INSERT INTO analysis_meter_results (
    analysis_run_id, meter_id, computed_status, evaluated_readings, flagged_readings, findings_count
) VALUES (
    @analysis_run_id, @meter_id, @computed_status, @evaluated_readings, @flagged_readings, @findings_count
);

-- name: ListAnalysisReadings :many
-- Engine input. NUMERIC → float8 rounds to the nearest double, exactly as
-- parsing the source decimal text does.
SELECT meter_id, reading_timestamp,
       consumption_kwh::float8 AS consumption_kwh,
       voltage_v::float8       AS voltage_v,
       current_a::float8       AS current_a,
       power_factor::float8    AS power_factor
FROM readings
ORDER BY meter_id, reading_timestamp;

-- name: ListAnalysisEvents :many
SELECT meter_id, event_timestamp, event_type, description
FROM events
ORDER BY meter_id, event_timestamp, id;

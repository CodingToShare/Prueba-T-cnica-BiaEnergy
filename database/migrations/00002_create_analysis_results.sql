-- Analysis state (Phase 03): runs executed in the background and the results
-- of each completed run. Source tables (meters, readings, events) are not
-- changed. System instants are timestamptz; episode times of a finding are
-- source wall-clock values and stay timezone-naive like readings (ADR-008).

-- +goose Up
CREATE TABLE analysis_runs (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    status              text        NOT NULL DEFAULT 'QUEUED',
    stage               text        NOT NULL DEFAULT 'QUEUED',
    progress_percent    smallint    NOT NULL DEFAULT 0,
    engine_version      text        NOT NULL,
    configuration       jsonb       NOT NULL,
    meters_count        integer,
    readings_count      integer,
    events_count        integer,
    findings_count      integer,
    high_priority_count integer,
    aggregate_confidence double precision,
    error_code          text,
    error_message       text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    started_at          timestamptz,
    completed_at        timestamptz,
    CONSTRAINT analysis_runs_status_valid
        CHECK (status IN ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED')),
    CONSTRAINT analysis_runs_stage_valid
        CHECK (stage IN ('QUEUED', 'LOADING_DATA', 'ANALYZING', 'PERSISTING_RESULTS', 'COMPLETED', 'FAILED')),
    CONSTRAINT analysis_runs_progress_range
        CHECK (progress_percent BETWEEN 0 AND 100),
    CONSTRAINT analysis_runs_counts_not_negative
        CHECK (meters_count >= 0 AND readings_count >= 0 AND events_count >= 0
               AND findings_count >= 0 AND high_priority_count >= 0),
    CONSTRAINT analysis_runs_aggregate_confidence_range
        CHECK (aggregate_confidence >= 0 AND aggregate_confidence <= 1),
    CONSTRAINT analysis_runs_engine_version_not_blank
        CHECK (btrim(engine_version) <> ''),
    CONSTRAINT analysis_runs_failure_has_error
        CHECK ((status = 'FAILED') = (error_code IS NOT NULL AND error_message IS NOT NULL)),
    CONSTRAINT analysis_runs_terminal_has_completed_at
        CHECK ((status IN ('COMPLETED', 'FAILED')) = (completed_at IS NOT NULL)),
    CONSTRAINT analysis_runs_completed_has_result
        CHECK (status <> 'COMPLETED'
               OR (findings_count IS NOT NULL AND high_priority_count IS NOT NULL AND progress_percent = 100))
);

-- At most one active (QUEUED or RUNNING) run: the database enforces the
-- one-analysis-at-a-time policy even when requests race.
CREATE UNIQUE INDEX analysis_runs_single_active
    ON analysis_runs ((true)) WHERE status IN ('QUEUED', 'RUNNING');

-- Serves "latest completed run", which every current-state read uses.
CREATE INDEX analysis_runs_latest_completed
    ON analysis_runs (completed_at DESC, id DESC) WHERE status = 'COMPLETED';

CREATE TABLE anomalies (
    id                        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    analysis_run_id           bigint           NOT NULL REFERENCES analysis_runs (id),
    meter_id                  text             NOT NULL REFERENCES meters (meter_id),
    priority                  integer          NOT NULL,
    type                      text             NOT NULL,
    severity                  text             NOT NULL,
    confidence                double precision NOT NULL,
    rule                      text             NOT NULL,
    recommended_action        text             NOT NULL,
    reason                    text             NOT NULL,
    started_at                timestamp        NOT NULL,
    last_observed_at          timestamp        NOT NULL,
    duration_seconds          bigint           NOT NULL,
    consumption_deviation_pct double precision NOT NULL,
    status                    text             NOT NULL DEFAULT 'OPEN',
    evidence                  jsonb            NOT NULL,
    created_at                timestamptz      NOT NULL DEFAULT now(),
    CONSTRAINT anomalies_priority_per_run UNIQUE (analysis_run_id, priority),
    CONSTRAINT anomalies_priority_positive CHECK (priority >= 1),
    CONSTRAINT anomalies_type_valid
        CHECK (type IN ('REAL_ANOMALY', 'EXPLAINABLE_ANOMALY', 'FALSE_POSITIVE', 'DATA_QUALITY')),
    CONSTRAINT anomalies_severity_valid CHECK (severity IN ('HIGH', 'MEDIUM', 'LOW')),
    CONSTRAINT anomalies_confidence_range CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT anomalies_text_not_blank
        CHECK (btrim(rule) <> '' AND btrim(recommended_action) <> '' AND btrim(reason) <> ''),
    CONSTRAINT anomalies_episode_order CHECK (last_observed_at >= started_at),
    CONSTRAINT anomalies_duration_positive CHECK (duration_seconds > 0),
    CONSTRAINT anomalies_status_valid CHECK (status IN ('OPEN')),
    CONSTRAINT anomalies_evidence_object CHECK (jsonb_typeof(evidence) = 'object')
);

-- Per-run analytical meter status, as computed by the engine (TD-07); never
-- the source status column.
CREATE TABLE analysis_meter_results (
    analysis_run_id    bigint  NOT NULL REFERENCES analysis_runs (id),
    meter_id           text    NOT NULL REFERENCES meters (meter_id),
    computed_status    text    NOT NULL,
    evaluated_readings integer NOT NULL,
    flagged_readings   integer NOT NULL,
    findings_count     integer NOT NULL,
    PRIMARY KEY (analysis_run_id, meter_id),
    CONSTRAINT analysis_meter_results_status_valid CHECK (computed_status IN ('OK', 'ALERT', 'CRITICAL')),
    CONSTRAINT analysis_meter_results_counts_not_negative
        CHECK (evaluated_readings >= 0 AND flagged_readings >= 0 AND findings_count >= 0)
);

-- +goose Down
DROP TABLE analysis_meter_results;
DROP TABLE anomalies;
DROP TABLE analysis_runs;

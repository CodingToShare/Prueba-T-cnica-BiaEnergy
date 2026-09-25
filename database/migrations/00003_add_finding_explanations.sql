-- Phase 05: operator-facing explanations of each finding (ADR-006). The
-- explanation is generated once, during the analysis run, and persisted with
-- its provenance; reading an anomaly never calls a provider. Deterministic
-- fields (type, severity, confidence, reason, recommended_action, evidence)
-- are unchanged and remain authoritative. Findings stored before this
-- migration have no explanation (all explanation columns NULL).

-- +goose Up
ALTER TABLE analysis_runs DROP CONSTRAINT analysis_runs_stage_valid;
ALTER TABLE analysis_runs ADD CONSTRAINT analysis_runs_stage_valid
    CHECK (stage IN ('QUEUED', 'LOADING_DATA', 'ANALYZING', 'GENERATING_EXPLANATIONS',
                     'PERSISTING_RESULTS', 'COMPLETED', 'FAILED'));

-- How the run's explanations were produced (provider, model, prompt version).
-- Never URLs or credentials. NULL for runs created before Phase 05.
ALTER TABLE analysis_runs ADD COLUMN explanation_configuration jsonb;
ALTER TABLE analysis_runs ADD CONSTRAINT analysis_runs_explanation_configuration_object
    CHECK (explanation_configuration IS NULL OR jsonb_typeof(explanation_configuration) = 'object');

ALTER TABLE anomalies
    ADD COLUMN explanation                jsonb,
    ADD COLUMN explanation_source         text,
    ADD COLUMN explanation_model          text,
    ADD COLUMN explanation_prompt_version text,
    ADD COLUMN explanation_generated_at   timestamptz,
    ADD COLUMN explanation_fallback_used  boolean,
    ADD COLUMN explanation_fallback_code  text;

ALTER TABLE anomalies
    ADD CONSTRAINT anomalies_explanation_source_valid
        CHECK (explanation_source IN ('DETERMINISTIC', 'OLLAMA')),
    ADD CONSTRAINT anomalies_explanation_object
        CHECK (explanation IS NULL OR jsonb_typeof(explanation) = 'object'),
    -- An explanation is stored with its whole provenance, or not at all.
    ADD CONSTRAINT anomalies_explanation_complete
        CHECK ((explanation IS NULL) = (explanation_source IS NULL)
           AND (explanation IS NULL) = (explanation_prompt_version IS NULL)
           AND (explanation IS NULL) = (explanation_generated_at IS NULL)
           AND (explanation IS NULL) = (explanation_fallback_used IS NULL)),
    -- A model is recorded only for generated text; a fallback always has a code
    -- and is always deterministic text.
    ADD CONSTRAINT anomalies_explanation_model_only_generated
        CHECK (explanation_model IS NULL OR explanation_source = 'OLLAMA'),
    ADD CONSTRAINT anomalies_explanation_fallback_code
        CHECK ((explanation_fallback_used IS TRUE) = (explanation_fallback_code IS NOT NULL)),
    ADD CONSTRAINT anomalies_explanation_fallback_deterministic
        CHECK (explanation_fallback_used IS NOT TRUE OR explanation_source = 'DETERMINISTIC');

-- +goose Down
ALTER TABLE anomalies
    DROP COLUMN explanation_fallback_code,
    DROP COLUMN explanation_fallback_used,
    DROP COLUMN explanation_generated_at,
    DROP COLUMN explanation_prompt_version,
    DROP COLUMN explanation_model,
    DROP COLUMN explanation_source,
    DROP COLUMN explanation;

ALTER TABLE analysis_runs DROP COLUMN explanation_configuration;

-- A run caught in the new stage is reported in the closest earlier stage.
UPDATE analysis_runs SET stage = 'ANALYZING' WHERE stage = 'GENERATING_EXPLANATIONS';
ALTER TABLE analysis_runs DROP CONSTRAINT analysis_runs_stage_valid;
ALTER TABLE analysis_runs ADD CONSTRAINT analysis_runs_stage_valid
    CHECK (stage IN ('QUEUED', 'LOADING_DATA', 'ANALYZING', 'PERSISTING_RESULTS', 'COMPLETED', 'FAILED'));

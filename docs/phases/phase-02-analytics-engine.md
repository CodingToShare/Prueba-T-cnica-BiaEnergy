# Phase 02 — Analytics / Anomaly Engine

- Status: **Planned / Not Started**
- Authorization: requires explicit user authorization.

## Objective

A pure, deterministic Go engine in `internal/analysis` that turns readings and events into classified, prioritized findings with severity, confidence, structured evidence, and a deterministic reason and recommended action — passing the four acceptance scenarios through generalized logic.

## Prerequisites

- Phase 01 Complete.
- Phase Entry Gate: backend build and static checks, Phase 01 unit tests, PostgreSQL integration tests, ingestion/data-integrity checks.

## Scope

- Data validation and per-reading quality flags; the engine distinguishes **anomalous measured behavior** from **unreliable/inconsistent measurements** (fundamental to M-112).
- Hour-aware robust baselines; robust Z and percentage deviation; persistence; multivariate and physical-consistency signals; event correlation; classification; severity; confidence; priority; evidence; deterministic reason and recommended action.
- One configuration structure for all thresholds and weights.
- Calibrate against the supplied data; resolve OD-01–OD-05, OD-08, OD-09, OD-11, OD-17, OD-18 (and the engine-owned parts of OD-06, OD-10), recording rationale.
- Define how analytical `computed_status` is derived, distinct from `source_status`.

## Non-Goals

HTTP endpoints, persistence of results, background runs, UI, LLM integration. No meter-ID-specific production rules.

## Expected Validation (strongest unit-level investment)

- Unit tests: robust statistics (median, MAD, spread floor), baseline construction, percentage deviation, persistence, multivariate signals, physical consistency, event correlation, classification, severity, confidence, priority, evidence completeness, action coherence.
- Acceptance tests on the supplied data: M-104 `EXPLAINABLE_ANOMALY`/`MEDIUM`; M-106 `FALSE_POSITIVE`/`LOW`; M-109 `REAL_ANOMALY`/`HIGH`; M-112 `DATA_QUALITY`/`HIGH`; M-109 prioritized first; each supported by evidence assertions (see testing strategy §9).
- Event-correlation effect test (without the explaining event, the M-106 deviation is not a false positive).
- Perturbation test and control-meter check.
- Regression: all Phase 01 checks.

## Evidence Required

Entry Gate result; exact commands and counts; calibration notes with rationale for every threshold; code review confirming no dataset-literal branching; Exit Gate checks; traceability rows FR-DET-*, FR-AI-001/002 (engine), BR-01–BR-08 updated.

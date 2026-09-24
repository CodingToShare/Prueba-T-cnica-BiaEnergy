---
name: analytics-engine
description: Implement or calibrate the deterministic anomaly engine — baselines, detection, persistence, multivariate and data-quality signals, event correlation, classification, severity, confidence, priority, evidence.
---

# Analytics Engine

## Use When

Any change to `internal/analysis` engine logic or its configuration, or to the semantics in `docs/ai/anomaly-analysis.md`.

## Responsibilities

Produce deterministic, generalized, evidence-backed findings (ADR-004) that satisfy BR-01–BR-08 and the acceptance scenarios.

## Required Rules

- Pure package: inputs are readings/events and configuration; no DB, HTTP, clock, network, or randomness.
- Meter-specific, hour-of-day-aware robust baselines (median, scaled MAD with a spread floor); robust Z and percentage deviation; persistence distinguishes spikes from sustained shifts.
- Data quality is evaluated before consumption anomalies; physical consistency is calibrated per meter.
- Events explain only when temporally aligned and semantically compatible; a "no event reported" record is not an explanation; data-quality events only corroborate.
- Confidence is computed from signals and its components stored as evidence; severity and priority are computed, not looked up by type.
- All thresholds/weights in one configuration struct; each documented with rationale when calibrated.
- Every finding carries complete evidence and an action coherent with its classification.

## Prohibited

Branching on meter IDs, timestamps, or dataset literals; tuning until tests pass without a documented generalizable rationale; using `expected_results.csv`; LLM calls; hard-coded confidence values.

## Completion Checklist

- Stage unit tests and the four acceptance tests pass; M-109 ranks first.
- Perturbation and control-meter checks pass.
- Calibration decisions recorded in `docs/ai/anomaly-analysis.md` and the decision register.

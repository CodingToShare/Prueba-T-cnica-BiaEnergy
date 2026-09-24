# ADR-004: Deterministic Anomaly Engine As Source Of Truth

- Status: Accepted
- Date: 2026-09-24

## Context

Most of the AI-specific score depends on correctly detecting, classifying, and prioritizing four scenarios, and on explaining conclusions with evidence. Results must be reproducible for reviewers, testable in CI, and robust to anomalies that contaminate their own baselines. An LLM cannot guarantee any of this.

## Decision

Anomaly detection, classification, severity, confidence, evidence, and priority are produced by a deterministic Go engine (`internal/analysis`) that:

- is a pure package: inputs are readings and events, output is findings with evidence; no HTTP, database, clock, or network access inside the engine;
- uses meter-specific, hour-of-day-aware robust baselines (median, MAD, robust Z-score, percentage deviation) plus persistence, multivariate, data-quality, and event-correlation stages as described in `docs/ai/anomaly-analysis.md`;
- keeps all thresholds and weights in one named configuration structure with documented rationale;
- derives confidence from signals (detection strength, persistence, multivariate confirmation, event-correlation certainty, data reliability), never from a constant;
- never branches on meter identifiers or dataset literals.

The same inputs and configuration always produce the same findings.

## Consequences

- Classifications are explainable by construction and verifiable with unit and acceptance tests.
- Calibration on a single small dataset risks overfitting; mitigations are robust statistics, documented rationale for each threshold, and a perturbation test (renamed meters, shifted timestamps) that must yield the same classifications.
- The engine can later run in a separate worker without changes (ADR-007 evolution path).

## Alternatives Considered

- **LLM-based detection or classification**: non-deterministic, hard to test, can hallucinate evidence, and unavailable offline.
- **Isolation Forest / trained ML model**: needs a Python service or Go ML libraries, is harder to explain per finding, and has little data to learn from.
- **Plain mean/standard-deviation Z-score**: sensitive to the very anomalies it must detect.

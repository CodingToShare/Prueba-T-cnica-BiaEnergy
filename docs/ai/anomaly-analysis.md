# Anomaly Analysis

How the platform decides whether a meter behaves abnormally, what kind of abnormality it is, how severe it is, and how sure the system is. This is product design, not implementation: **no thresholds are frozen** until Phase 02 calibrates them. Governing decision: ADR-004.

## 1. Principles

- Deterministic and reproducible: same inputs and configuration give the same findings.
- Generalized: rules operate on signals, never on meter identifiers or dataset literals.
- Robust: baselines resist contamination by the anomalies being detected.
- Evidence-first: every conclusion is backed by structured, displayable evidence.
- One configuration location for every threshold and weight, each with a documented rationale.

## 2. Pipeline

| Stage | Purpose | Output |
| --- | --- | --- |
| Data validation | Check completeness, duplicates, ranges, and physical plausibility before trusting values | Per-reading quality flags; per-meter reliability |
| Baseline | Expected behavior per meter and hour of day | Median and robust spread per (meter, hour) for each variable |
| Statistical detection | Measure how far each reading is from its baseline | Robust Z-score and percentage deviation per reading/variable |
| Persistence | Distinguish isolated spikes from sustained shifts | Episode start, duration, share of affected hours, level shift |
| Multivariate | Check whether electrical variables corroborate or contradict the consumption change | Changed variables and consistency signals |
| Event correlation | Align episodes with known events | Matched events, alignment quality, semantic compatibility |
| Classification | Assign one of four types | `REAL_ANOMALY`, `EXPLAINABLE_ANOMALY`, `FALSE_POSITIVE`, `DATA_QUALITY` |
| Severity | Impact level | `HIGH`, `MEDIUM`, `LOW` |
| Confidence | Certainty of the conclusion | Number in [0, 1] plus a qualitative band |
| Evidence | Facts supporting the conclusion | Structured evidence (see `explainability.md`) |
| Recommendation | Next action coherent with the classification | Action code + deterministic text |
| Explanation (optional) | Operator-friendly wording | Text from an `ExplanationProvider` |

## 3. Robust Baseline

- **Meter-specific**: each meter has its own scale (hourly consumption across the fleet spans roughly 13–120 kWh).
- **Hour-aware**: consumption follows a strong daily profile (night, morning ramp, working hours, evening), so each hour of day has its own baseline. With 14 days there are ~14 samples per hour but only ~2 per weekday-hour, so weekday granularity is not planned (OD-02).
- **Robust statistics**: median for center; MAD scaled to a standard-deviation equivalent (×1.4826) for spread; robust Z = (x − median) / scaled MAD; percentage deviation = (x − median) / median. A floor on spread avoids division by near-zero MAD.
- **Contamination**: a sustained shift covering part of the period drags even robust statistics. Candidate mitigations (OD-02): compute baselines from a reference window, or iterate after excluding detected episodes. The chosen approach and its rationale are recorded in Phase 02.
- **Data-quality-flagged readings** may be excluded from baselines (OD-18).

## 4. Detection Signals

| Capability (§8) | Candidate signal |
| --- | --- |
| Spikes / abrupt changes | Large single-hour robust Z or step change between consecutive levels |
| Persistent change vs baseline | Sustained share of hours beyond threshold; shift in daily totals after a change point |
| Outliers | Extreme robust Z on any variable |
| Abnormal hourly patterns | Change in the shape of the hour-of-day profile (not only its level) |
| Data-quality problems | Missing/duplicate timestamps, out-of-range values, physically inconsistent combinations, erratic electrical jumps while consumption is stable |
| Anomalous variable relationships | Current not tracking consumption, power-factor shift, voltage shift, broken consumption-to-power consistency |
| Explained changes (false positives) | Deviation aligned with a compatible known event |

**Physical consistency** is a key generalizable signal: under normal operation, hourly energy is approximately proportional to `voltage × current × power_factor`. In normal readings the ratio is stable per meter but not exactly 1 (so it must be calibrated per meter, not asserted as a physical identity). A reading whose electrical variables break this relationship while consumption stays on its profile points to measurement problems rather than a consumption anomaly.

## 5. Event Correlation

Events provide context; they never replace data-derived signals.

- **Temporal alignment**: an episode onset close to an event start supports correlation.
- **Semantic compatibility**: the event type must be able to explain the observed change (e.g., a scheduled outage explains a drop, an operational change can explain a sustained level change). The mapping is OD-08.
- **Window**: events have only a start time; durations appear in free text. How the influence window is determined is OD-09.
- **Non-explanatory events**: an event row stating that no operational event occurred (type `UNKNOWN` in the dataset) is not an explanation; it confirms the absence of one.
- **Data-quality events** corroborate a data-quality finding but cannot create one on their own.

## 6. Classification Semantics

| Type | Meaning | Typical pattern |
| --- | --- | --- |
| `DATA_QUALITY` | Readings cannot be trusted | Electrical variables implausible or mutually inconsistent while consumption remains normal |
| `FALSE_POSITIVE` | A statistical deviation fully accounted for by a known event | Deviation confined to a compatible event's window and returns to baseline |
| `EXPLAINABLE_ANOMALY` | A real, persistent change that a known event explains | New sustained level beginning at a compatible event; operation and baseline should be validated |
| `REAL_ANOMALY` | Significant, persistent deviation with no explanation | Sustained deviation corroborated by electrical changes and no explanatory event |

Intended evaluation order: data quality first (untrustworthy readings must not produce consumption anomalies), then deviation + persistence, then event correlation to separate explained from unexplained changes. The exact decision rules are defined and tested in Phase 02. An `INVESTIGATE`/`UNKNOWN` type is not planned (OD-11).

## 7. Severity, Priority And Confidence

- **Severity** reflects potential impact: magnitude of deviation, persistence, electrical corroboration, and whether the finding needs action. It is computed, not looked up from the type (OD-04).
- **Priority** orders the investigation queue. A high-severity real anomaly ranks first; equal-severity ties follow a documented order (OD-05).
- **Confidence** combines, with weights set in Phase 02 (OD-03):
  - detection strength (how extreme the deviation is),
  - persistence (how sustained),
  - multivariate confirmation (how many variables agree),
  - event-correlation certainty (alignment and semantic fit, or clear absence of any explanation),
  - data reliability (penalized when inputs are unreliable, except for data-quality findings where unreliability is the finding).
  Confidence is never a hard-coded constant; its components are stored as evidence.

## 8. Acceptance Scenarios

Verified by tests, reached by generalized logic. Observations below come from Phase 00 inspection of the supplied files and must be confirmed with computed statistics in Phases 01–02.

| Meter | Observation | Expected |
| --- | --- | --- |
| M-104 | From 2026-09-11 00:00 consumption rises roughly 45–50% at every hour and stays there; current rises proportionally; voltage and power factor broadly stable. An `OPERATIONAL_CHANGE` event ("new production line") starts at the same time. | `EXPLAINABLE_ANOMALY` / `MEDIUM` |
| M-106 | On 2026-09-08 from 00:00 to 11:00 consumption and current fall to roughly a fifth of normal, then return. A `SCHEDULED_OUTAGE` event (described as 12 hours) starts at the same time. | `FALSE_POSITIVE` / `LOW` |
| M-109 | From 2026-09-12 14:00 consumption and current roughly double and stay high; power factor drops from ~0.94 to ~0.74; voltage dips slightly. The only event row states that no operational event was reported. | `REAL_ANOMALY` / `HIGH`, ranked first |
| M-112 | From 2026-09-13 00:00 about every third hour voltage jumps to ~202 V or ~240 V, current and power factor take implausible values, while consumption stays on its normal profile. A `DATA_QUALITY` event starts at the same time. | `DATA_QUALITY` / `HIGH` |

Calibration targets (not challenge requirements): the eight remaining meters should not produce escalated findings; a perturbation test (renamed meters, shifted timestamps) must yield identical classifications.

## 9. Open Calibration Questions

Thresholds (OD-01), baseline window and contamination (OD-02), confidence weights (OD-03), severity (OD-04), priority (OD-05), consumption windows (OD-06), event semantics and windows (OD-08, OD-09), computed meter status (OD-10), finding granularity (OD-17), exclusion of low-quality readings (OD-18). Each resolution is recorded in `docs/product/assumptions-and-decisions.md` and summarized here with its rationale.

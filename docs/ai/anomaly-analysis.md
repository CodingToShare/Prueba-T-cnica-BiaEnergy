# Anomaly Analysis

How the platform decides whether a meter behaves abnormally, what kind of abnormality it is, how severe it is, and how sure the system is. Governing decision: ADR-004.

**Implemented in Phase 02** as the pure Go package `src/backend/internal/analysis`: `analysis.New(analysis.DefaultConfig())`, then `Engine.Analyze(ctx, readings, events)`, which returns findings in priority order plus a per-meter summary. The engine has no database, HTTP, clock, network, randomness or concurrency. Nothing calls it yet: persistence of results, analysis runs and the API are Phase 03, and generative explanation is Phase 05.

## 1. Principles

- **Deterministic and reproducible:** same inputs and configuration give the same result, regardless of input order (tested with shuffled input).
- **Generalized:** rules act on signals and structured event types, never on meter identifiers, timestamps, or event descriptions.
- **Robust:** baselines use median and MAD, and exclude anomalous readings.
- **Evidence-first:** every finding carries the structured evidence behind it.
- **One configuration location:** `analysis.Config` / `DefaultConfig()` holds every threshold and weight (§12). It has no package-level mutable state.

## 2. Input Assumptions And Validation

- **Readings:** hourly, one per meter and timestamp. The timestamp is the source wall clock (ADR-008); its hour of day selects the baseline.
- **Events:** `meter_id`, `timestamp`, `type`, and a description kept verbatim. Any type string is accepted.
- **Rejected with `ErrInvalidInput`:**
  - an empty reading set;
  - a missing meter ID or timestamp;
  - a non-finite measurement;
  - duplicate meter/timestamp pairs;
  - readings of a meter that are not a whole number of hours apart (gaps are allowed);
  - events without a meter, time or type.
- Physically unusual but finite values (zero, negative) are analyzed, not rejected; Phase 01 keeps them for this reason.
- The engine copies and sorts its inputs; caller slices are never modified. Events for meters without readings are ignored. A cancelled context stops the analysis.

## 3. Pipeline

| Stage | What it does | Output |
| --- | --- | --- |
| Validation | Structural checks at the engine boundary (§2) | Error, or per-meter sorted series |
| Baseline | Per meter and hour of day: median and MAD of recent non-flagged same-hour readings (§4) | Baseline per reading and metric |
| Signals | Dual gate on five metrics (§5, §6) | Reading-level signals |
| Reading interpretation | Load change, inconsistent measurement, or neither (§7) | Reading kind |
| Episodes | Group flagged readings of one kind; measure persistence and recovery (§8) | Episodes |
| Event correlation | Events near the onset, each with a role (§9) | Related events |
| Classification | Four canonical types (§10) | Type + rule |
| Severity | From type and evidence (§11) | `HIGH` / `MEDIUM` / `LOW` |
| Confidence | Weighted evidence components (§12) | [0, 1] + breakdown |
| Priority | Deterministic comparator (§13) | Ordered findings |
| Recommendation + reason | Action per type; one sentence built from evidence (§14) | Action code + text |
| Meter summary | Counts and computed status (§14) | `OK` / `ALERT` / `CRITICAL` |

## 4. Historical Baseline

- **Scope:** per meter and **hour of day**. The EDA confirmed a strong daily profile (hour-of-day consumption medians span, e.g., 22.6–36.9 kWh on one meter), so hours are not pooled. With 14 days there are too few samples per weekday-hour for weekday profiles.
- **Window:** the 7 most recent same-hour readings of that meter that were **not flagged**. The baseline is evaluated only with exactly 7 such observations, so the first evaluated day is the eighth. In normal operation this equals "the same hour on the previous 7 days".
- **Deviation from the plain previous-7-days window (documented decision, OD-02):** flagged readings are skipped. An ongoing episode is therefore compared with its pre-episode behavior (a frozen baseline), and a finished episode does not contaminate later baselines. On the supplied data, a plain rolling window would have absorbed a sustained shift: by the fourth day of the M-104 shift, 13 of its 96 hours fell under the statistical gate because anomalous days had inflated the MAD. With the implemented window all 96 stay detected. This is tested with a synthetic 12-day shift that outlasts the window.
- **No future leakage:** a reading's baseline uses only earlier readings. Tests prove it: evaluating a prefix gives the same results as evaluating the full series, and changing future readings changes no earlier result.
- **Robust statistics:** median; MAD = median |x − median|; robust Z = 0.6745 × (x − median) / max(MAD, 0.001 × |median|); relative deviation = (x − median) / |median|.
  - The spread floor keeps the Z finite when the MAD is zero.
  - When the median is zero, the relative deviation is undefined, so that metric is not evaluated for the reading.
  - A reading whose consumption cannot be evaluated is not judged at all.
- **Data-quality readings are excluded from baselines** (OD-18), like every flagged reading.

## 5. Dual-Gate Signals And Calibration

A metric of a reading is a **signal** only if **both** conditions hold:

- |relative deviation| ≥ the metric's threshold;
- |robust Z| ≥ 3.5.

Robust Z alone is not usable here. The independent audit reproduced maximum |Z| on the eight event-free meters of 22.85 consumption, 12.96 voltage, 24.39 current, 17.54 power factor and 29.41 ratio, using the implemented spread floor. The earlier EDA range of 17–48 was not the implemented floored-Z maximum. Tiny day-to-day spreads explain why a Z gate of 3.5 alone can flag normal hours. The relative gate separates meaningful change from noise; the Z gate discards changes within a noisy hour's normal spread.

**Calibration.** The exploratory analysis used only `readings.csv` and `events.csv`, never evaluator material. For every mature reading (days 8–14) it measured the relative deviation against the trailing same-hour baseline. The distribution for the eight meters that have no events (1,344 meter-hours) sets the thresholds:

| Metric | Largest deviation on the 8 event-free meters | Default threshold | Margin |
| --- | --- | --- | --- |
| Consumption | 18.4% | **25%** | 1.36× |
| Voltage | 2.2% | **3%** | 1.36× |
| Current | 13.9% | **20%** | 1.44× |
| Power factor | 5.3% | **8%** | 1.5× |
| Consumption-to-load ratio | 26.9% | **40%** | 1.49× |
| Robust Z (all metrics) | — | **3.5** | Conventional robust-outlier cut-off |

Each default is a simple rounded value above the largest normal deviation of any stable meter-hour. The values are the same for every meter, hour and event type. The starting hypotheses in the Phase 02 brief (consumption 20–25%, voltage 3%, current 15–20%, PF 7–8%, Z 3–4) were confirmed, except that the tighter ends (20% consumption, 15% current, 7% PF) leave less than 1.35× margin over the stable maxima.

**Sensitivity:** changing each relative threshold, robust-Z threshold, event window, sustained duration or high-deviation threshold separately by ±20% preserves the four classifications, severities and their order. Moving all five relative thresholds together by −20% or +20% also preserves that outcome; confidence and detailed signals can change. **The gap is an exception:** 3 h → 2.4 h splits the intermittent every-third-hour data-quality pattern into singletons, leaving three findings; 3 h → 3.6 h preserves four. This is an intentional episode-continuity boundary, not evidence of general calibration robustness. Counts and confidence saturation settings are not covered by the single-threshold claim.

## 6. Derived Electrical Consistency Feature

`consumption_to_load_proxy_ratio = consumption_kwh / (voltage_v × current_a × power_factor / 1000)`.

- It is a **relative consistency feature**, not a power-flow model. Wiring, phase topology and conversion factors are unknown, so the ratio is never compared with 1 or any physical constant. It is baselined per meter and hour like the other metrics; stable meters show ratios of about 1.02–1.12, each stable over time.
- It is unavailable, and not evaluated, when the denominator is non-positive or non-finite, or the ratio is not finite. Other metrics of that reading are still analyzed, and a zero current or voltage shows up as a signal on that metric.

## 7. Reading Interpretation

Each evaluated reading is interpreted from its signals. Consumption is never reinterpreted as a measurement problem.

| Kind | Condition |
| --- | --- |
| Load change (up or down) | Consumption is a signal |
| Inconsistent measurement | Consumption is **not** a signal, but at least 2 of voltage, current, power factor and ratio are |
| Other flagged | Any other single signal. Not grouped into episodes, but still excluded from baselines and counted in the meter summary |

Rationale: current tracks load, so repeated electrical changes while consumption stays on its profile suggest measurement inconsistency. Requiring two feature signals controls false positives. A sufficiently large drift in one measured variable can also trigger its derived ratio and become reportable; these are two consistency features, not two independent measurements. The ratio is retained as evidence but adds no independent confidence vote.

## 8. Persistence And Episodes

- **Grouping:** flagged readings of the **same kind** form an episode when consecutive flagged readings are at most **3 h** apart (`MaxGap`). Up to two normal hourly readings are bridged, so a fault recurring every third hour stays one episode; three intervening normal readings (4 h between flags) split it. Load-up, load-down and inconsistency episodes never merge.
- **Reportable:** at least **3** flagged readings (`MinEpisodeReadings`). A one- or two-reading spike is detected but never reported.
- **Measured:** start, last flagged reading, elapsed duration (first to last plus one hour), flagged readings, observed span readings, density (flagged/observed span), longest consecutive run. Missing timestamps do not become observed samples: five flags spaced 3 h apart span 13 h, with longest run 1 and density 1 among the five observations. Density is not time coverage.
- **Sustained:** duration ≥ **12 h** (half a daily cycle).
- **Recovery:** immediately after the episode, **6** consecutive evaluated readings one configured interval apart without the episode's signal: consumption no longer deviating for load episodes, or consistent readings for inconsistency episodes. A missing timestamp, unevaluated reading, renewed signal, or end of data stops the sequence; recovery is not inferred across the interruption. The median consumption deviation over the observed sequence is kept as evidence. Six readings cover more than `MaxGap`, so a bridged gap is never mistaken for recovery.

## 9. Event Correlation (OD-08, OD-09)

- Events of the same meter within **±3 h** of the episode onset are related; the lookup is a binary search per meter. More distant events are not correlated.
- Decisions use only the structured **type** and the observed behavior. Descriptions are kept verbatim as evidence and are never parsed. For example, an outage's "12 hours" text is not used: the observed recovery decides.

| Event type | Load rise | Load drop | Inconsistent measurement |
| --- | --- | --- | --- |
| `OPERATIONAL_CHANGE` | Explains | Explains | Context |
| `SCHEDULED_OUTAGE` | Context (cannot explain a rise) | Explains | Context |
| `DATA_QUALITY` | Context | Context | Corroborates (never creates a finding) |
| `UNKNOWN` ("no operational event reported") | Context | Context | Context |
| Any other type | Context | Context | Context |

All related events are preserved with their offset from the onset and their role.

## 10. Classification (BR-01, BR-03–BR-05)

Rules, in order:

1. An inconsistency episode → `DATA_QUALITY` (`REPEATED_ELECTRICAL_INCONSISTENCY`). It needs no event, and an event alone never creates one.
2. A load episode with no explaining event → `REAL_ANOMALY` (`UNEXPLAINED_PERSISTENT_CONSUMPTION_SHIFT`). **Insufficient evidence** (kept internal, OD-11): if the episode is neither sustained nor corroborated by any electrical metric, it is not reported.
3. Explained by `SCHEDULED_OUTAGE`, with recovery observed → `FALSE_POSITIVE` (`TEMPORARY_DEVIATION_EXPLAINED_BY_SCHEDULED_OUTAGE`).
4. Explained by `SCHEDULED_OUTAGE`, without recovery → `EXPLAINABLE_ANOMALY` (`SCHEDULED_OUTAGE_WITHOUT_OBSERVED_RECOVERY`). The outage explains the start but not the persistence.
5. Explained by `OPERATIONAL_CHANGE` → `EXPLAINABLE_ANOMALY` (`CONSUMPTION_SHIFT_EXPLAINED_BY_OPERATIONAL_CHANGE`).

When several events explain, the one closest to the onset is used; ties use the earlier timestamp, then lexical type and description order. Description ordering only makes identical-type evidence deterministic; descriptions never determine event roles or classification. Findings keep their deviation evidence whatever the type; nothing is classified as "normal". Finding granularity (OD-17) is **one finding per reportable episode**; a meter may have several.

A load episode is **corroborated** by an electrical metric that is a signal in ≥ 50% of its flagged readings. Current must change in the same direction as consumption, without opposite-direction current signals in that episode. Voltage and power factor can provide ancillary support only when current meets that condition; their changes alone cannot confirm increased/decreased load. The load ratio never corroborates a load episode: a ratio shift means the electrical relationship changed, which is inconsistency evidence rather than confirmation of actual load.

## 11. Severity (OD-04)

Severity is operational importance, independent of confidence. It follows from type **and** evidence:

| Type | HIGH | MEDIUM | LOW |
| --- | --- | --- | --- |
| `REAL_ANOMALY` | Sustained **and** median consumption deviation ≥ 50% **and** ≥ 1 corroborating metric | Sustained **or** ≥ 50% | Otherwise |
| `DATA_QUALITY` | Sustained: unreliable measurements over a material period | Otherwise | — |
| `EXPLAINABLE_ANOMALY` | Never: an explained change is not escalated (BR-03) | Sustained: the change must be validated | Otherwise |
| `FALSE_POSITIVE` | — | — | Always: explained and recovered |

## 12. Confidence (OD-03)

Confidence is how well the evidence supports the classification. It is the weighted sum of five components, each in [0, 1], and every finding exposes the breakdown:

| Component | Weight | Definition |
| --- | --- | --- |
| Signal strength | 0.25 | (evidence strength − 1) / 2, clamped. Evidence strength is the median of \|primary deviation\| / threshold: consumption for load episodes, the strongest electrical metric for inconsistency episodes. It reaches 1 at 3× the threshold |
| Persistence | 0.20 | Flagged readings / 24, clamped |
| Multivariate support | 0.20 | Corroborating directly measured electrical metrics (V, I, PF) / 3, clamped. The derived ratio is evidence only, never an additional vote |
| Event context | 0.20 | Explained findings: 1 − 0.5 × \|event offset\| / 3 h. Real anomalies: 1, or 0.5 when an explanatory-type event that cannot explain this direction is near. Data quality: 1 with a corroborating event, 0.5 without |
| Pattern support | 0.15 | False positive: 1 − \|post-episode deviation\| / 25%. Data quality: 1 − \|median consumption deviation\| / 25% (stable consumption). Real and explainable: 1 while the shift persists, 0.5 once recovered |

Weights and saturation points are judgment calls, validated for monotonicity and bounds by tests: stronger evidence never lowers confidence, the result is always in [0, 1], and it is never NaN or infinite. Confidence is never a constant. A `FALSE_POSITIVE` can have high confidence while its severity is `LOW`.

Monotonicity applies within the same classification and event/recovery context. Persistence changes smoothly with flagged count (11 → 12 adds 0.20/24, not a confidence cliff). Intentional steps remain when a metric reaches the 50% corroboration share, recovery is established (a 0.075 weighted pattern change for real/explainable findings), or event context changes. This is a heuristic certainty-of-classification score, not a calibrated probability or an independence model; V, I and PF can themselves be correlated. Missing ratio evidence adds nothing; event absence has the class-specific values above. Pattern support represents consistency/reliability heuristically, not a separate source-quality probability.

## 13. Priority (OD-05)

`analysis.ComparePriority`, a transparent comparator rather than an opaque score:

1. Severity: `HIGH` → `MEDIUM` → `LOW`.
2. Classification by operational risk: `REAL_ANOMALY` → `DATA_QUALITY` → `EXPLAINABLE_ANOMALY` → `FALSE_POSITIVE`.
3. Confidence, descending.
4. Evidence strength, descending.
5. Earlier onset, then meter ID, only as a deterministic tie-breaker.

Findings are numbered from 1 in this order. An unexplained HIGH real anomaly therefore ranks ahead of a HIGH data-quality issue (BR-06).

## 14. Recommendation, Reason And Computed Status

- **Recommended action** (BR-08): `REAL_ANOMALY` → `INVESTIGATE_METER_AND_INSTALLATION`; `DATA_QUALITY` → `VALIDATE_MEASUREMENT_OR_SENSOR`; `EXPLAINABLE_ANOMALY` → `VALIDATE_OPERATIONAL_CHANGE`; `FALSE_POSITIVE` → `NO_ESCALATION_MONITOR`.
- **Reason:** one deterministic sentence formatted only from the finding's evidence: deviation, duration, onset, corroborating metrics, event role, and recovery. It is not a generative explanation (Phase 05).
- **Computed meter status** (OD-10, engine part): `CRITICAL` for a HIGH real anomaly; `ALERT` for any other real anomaly or any MEDIUM/HIGH finding (so a HIGH data-quality meter is an Alert, as in the challenge example); otherwise `OK`. `source_status` is never used. Phase 03 exposes it.
- **Consumption windows** (OD-06, engine part): each finding reports observed and baseline energy summed over its flagged readings, the deviation of those sums, and the median hourly deviation. List/KPI windows are Phase 03.

## 15. Output

`Result{Findings, Meters}`. Each `Finding` carries:

- priority, meter, type, rule, severity, confidence and its breakdown;
- action and reason;
- onset, last flagged reading, duration and evidence strength;
- consumption evidence and persistence (with recovery);
- per-metric evidence for all five metrics, including whether each corroborates;
- related events with roles;
- every reading-level signal (metric, time, observed, baseline, deviation, deviation %, robust Z, direction, strength).

See `explainability.md` §1.

## 16. Acceptance On The Supplied Data

Automated in `internal/analysis/dataset_acceptance_test.go`: CSV → Phase 01 parser → engine with `DefaultConfig()`. Results after the final change:

| Priority | Meter | Type / severity | Confidence | Key evidence |
| --- | --- | --- | --- | --- |
| 1 | M-109 | `REAL_ANOMALY` / `HIGH` | 0.93 | Consumption +110% (median) for 58 h from 2026-09-12 14:00; current +111% (same direction) and power factor −22% corroborate; the `UNKNOWN` event is context only; no recovery |
| 2 | M-112 | `DATA_QUALITY` / `HIGH` | 0.92 | 16 of 46 readings over 46 h inconsistent in voltage, current, power factor and the load ratio; consumption never deviates (within 9%, +1.6% over the episode); the `DATA_QUALITY` event corroborates |
| 3 | M-104 | `EXPLAINABLE_ANOMALY` / `MEDIUM` | 0.73 | Consumption +47% for 96 h, current +47%; the `OPERATIONAL_CHANGE` event is at the onset; still shifted at the end |
| 4 | M-106 | `FALSE_POSITIVE` / `LOW` | 0.76 | Consumption −80% for 12 h with current −80%; the `SCHEDULED_OUTAGE` event is at the onset; recovered from 2026-09-08 12:00 (within 2%) |

Also verified:

- **Control meters:** the eight meters without events have **zero flagged readings** and no finding; every meter has 168 evaluated readings.
- **Event effect:** removing the M-106 and M-104 events turns both into `REAL_ANOMALY`.
- **Perturbation:** renamed meters and timestamps shifted by 37 days + 5 h give identical types, severities, confidences and order.
- **Stronger perturbation:** renaming all meters and shifting by exactly 37 days preserves the complete result after translating IDs and timestamps back, including signals, metrics, reasons, recovery, confidence and meter summaries. Whole days preserve hour-of-day; the older +5 h test also passes this hourly-only implementation, but is not a general invariant for calendar-aware baselines.
- **Stability:** the ±20% threshold sensitivity described in §5.
- **Determinism:** repeated and shuffled runs give identical results.

Analysis of the 4,032 readings takes about 4–6 ms, from the benchmark `BenchmarkAnalyze_SuppliedDataset`. The dataset is tiny, so this says nothing about scale.

## 17. Limitations (Implemented Behavior)

- **The baseline never adapts after a legitimate permanent change.** An explained shift keeps being reported as long as it lasts. A 17-day synthetic explained shift verifies this behavior. Operator-confirmed or policy-driven re-baselining is a future product decision, not implemented. A single borderline flag followed by normal readings does not cause a self-reinforcing anomaly loop in the 25-day regression scenario.
- **The first 7 observations of each meter/hour are not evaluated** (seven days for complete hourly data); missing observations can extend this cold start. An anomaly in them would also enter the baseline, where only the median's robustness limits its effect.
- **Hour-of-day only;** no weekday or seasonal profiles.
- **Hourly readings are assumed;** other intervals need a configuration change and recalibration.
- **Not reported:** an electrical drift that triggers only one feature while consumption is stable (for example, a 15% power-factor decline with the ratio below its gate). A stronger single-variable drift that also triggers the ratio can be reported as data quality (§7).
- **A consumption change that the electrical metrics contradict** (current flat, ratio shifting) is reported as an uncorroborated `REAL_ANOMALY`, at most `MEDIUM`, rather than as `DATA_QUALITY`.
- **Hours with a zero consumption baseline are not evaluated.**
- **Events are correlated only near the onset;** event descriptions are never interpreted.
- **Calibration rests on one 14-day dataset.** Thresholds are heuristic MVP defaults, not production-calibrated; representative historical data is needed for later recalibration. Confidence weights are judgment, not learned. Intermittent findings depend on the episode gap (§5).

## 18. Future Ideas (Not Implemented)

Re-baselining on validated operational changes; weekday-aware baselines once longer history exists; reporting single-metric electrical drifts; incremental baselines for continuous ingestion. Each needs a demonstrated requirement.

# Phase 02 — Analytics / Anomaly Engine

- Status: **Complete**
- Authorization: explicitly authorized by the user, together with the local Phase 01 checkpoint commit. Phase 03 is not authorized.
- Date: 2026-09-24
- Phase 01 checkpoint: local commit `a81f38e` "feat: complete runtime foundation and dataset ingestion" (not pushed; no remote).

## Objective

A pure, deterministic Go engine in `internal/analysis` that turns readings and events into classified, prioritized findings with severity, confidence, structured evidence, a deterministic reason and a recommended action. It passes the four acceptance scenarios through generalized logic.

## Phase Entry Gate

| Check | Result |
| --- | --- |
| Phase 01 Complete; Phase 02 authorized | Yes. Phase 01 audited "Complete with non-blocking notes"; Phase 02 authorized explicitly |
| Checkpoint | Before committing: state matched the audit (17 modified, 3 deleted, 41 untracked, nothing staged); no binaries, coverage files, `.env`, dumps, secrets or expected-results files; the only secret-like hits were the documented local-only placeholder and a fake test value. Pre-commit regression passed (`mod verify`, `vet`, unit, integration 41/41, `diff --check`). Commit `a81f38e`; working tree clean afterwards; no remote |
| Baseline on `a81f38e` | `go build ./...` and `go vet -tags=integration ./...` OK; unit PASS (5 packages); integration 41 top-level tests PASS, 0 failed, 0 skipped |
| Governing documents read | AGENTS, project context, skills (phase-governance, architecture, analytics-engine, testing, code-review, documentation), this document, product docs, architecture, data model, ADR-004, `docs/ai/*`, testing strategy, DoD |

## Exploratory Data Analysis (readings.csv and events.csv only)

Deterministic scratch script, not committed. Baseline for every reading: the same meter and hour on the previous 7 days, so only past data is used.

- **Hourly pattern:** a strong daily profile on every meter (hour-of-day consumption medians span, e.g., 22.6–36.9 kWh on M-101). Hours are therefore not pooled.
- **Stable-period dispersion:** median relative MAD over same-hour histories is 2.4% for consumption, 0.3% for voltage, 1.1% for current, 0.9% for power factor, and 2.9% for the consumption-to-load ratio.
- **Largest deviation on the 8 event-free meters** (1,344 mature meter-hours): consumption 18.4%, voltage 2.2%, current 13.9%, power factor 5.3%, ratio 26.9%.
- **Robust Z on the same hours:** median about 0.9, 90th percentile about 3, 99th 7–9, maximum 17–48. Z alone would flag normal hours; this motivates the dual gate.
- **Relationships:**
  - Current tracks consumption; voltage stays within about ±2%.
  - consumption / (V·I·PF/1000) is stable per meter (about 1.02–1.12), not 1, so it can only be compared with each meter's own history.
- **Candidate persistence:**
  - Affected meters show 12, 58 and 96 consecutive anomalous hours.
  - One meter shows an intermittent pattern: 16 anomalous readings, every third hour, over 46 h. This sets the 3 h gap.
- **Event timing:** all 4 events are at the onset hour of the corresponding behavior. `UNKNOWN` states that no event was reported. The outage's duration exists only as free text.
- **A plain rolling baseline absorbs a sustained shift:** by the fourth day of the M-104 shift, 13 of its 96 hours fell under the Z gate because the MAD inflated. This motivated excluding flagged readings from baselines.

## Decisions Made

Resolved open decisions (OD-01–OD-06 engine part, OD-08–OD-11, OD-17, OD-18) are recorded with rationale in `docs/product/assumptions-and-decisions.md` and described in `docs/ai/anomaly-analysis.md`. Summary:

| Topic | Decision |
| --- | --- |
| Baseline | Per meter and hour of day: the 7 most recent **non-flagged** same-hour readings, all 7 required, so the eighth day is the first evaluated. This equals the previous 7 days in normal operation and freezes during an episode. Documented deviation from the plain 7-day window |
| Robust statistics | Median, MAD, robust Z = 0.6745 (x − median) / max(MAD, 0.1% of \|median\|); relative deviation undefined for a zero median |
| Dual gate | \|deviation\| ≥ consumption 25%, voltage 3%, current 20%, PF 8%, ratio 40%, **and** \|Z\| ≥ 3.5 |
| Reading kinds | Consumption signal → load change; normal consumption plus ≥ 2 electrical signals → inconsistent measurement |
| Episodes | Same kind; gap ≤ 3 h; ≥ 3 flagged readings to report; sustained ≥ 12 h; recovery = 6 normal readings |
| Events | ±3 h of onset; role by structured type and direction; descriptions never parsed; outage length taken from observed recovery |
| Classification, severity, confidence, priority, action, computed status | As in `anomaly-analysis.md` §10–§14 |
| Package | One package `internal/analysis`, standard library only; no new dependency, migration, table or sqlc |

## Implementation

| Area | Files (`src/backend/internal/analysis/`) |
| --- | --- |
| Model | `model.go`: input `Reading`, `Event`; typed `EventType`, `Metric`, `Direction`, `Classification`, `Severity`, `Action`, `Rule`, `EventRole`, `MeterStatus`; output `Result`, `Finding`, `ConsumptionEvidence`, `Persistence`, `Recovery`, `MetricEvidence`, `Signal`, `RelatedEvent`, `ConfidenceBreakdown`, `MeterSummary` |
| Configuration | `config.go`: `Config`, `DefaultConfig()`, `Validate()` (every setting, errors joined) |
| Robust statistics | `robust.go`: median, MAD, robust deviation with spread floor |
| Baseline and signals | `baseline.go`: features (incl. the guarded ratio), sequential per-meter evaluation, dual gate, reading kinds, signal evidence |
| Episodes | `episodes.go`: grouping, persistence, recovery, metric and consumption evidence, evidence strength |
| Events | `correlation.go`: per-meter sorted index with binary search, roles, explanation choice |
| Decisions | `classification.go`: assess, classify, severity, confidence, action, deterministic reason |
| Priority | `priority.go`: exported `ComparePriority`, numbering |
| Engine | `engine.go`: `New`, `Analyze`, boundary validation (`ErrInvalidInput`), cancellation, meter summary and status |

The engine is pure, with no DB, HTTP, clock, network, randomness or goroutines. It is not yet called by any command or endpoint (Phase 03).

## Original Implementation Tests (superseded by the independent audit below)

| Level | Command (repository root) | Result |
| --- | --- | --- |
| Analytics unit + acceptance | `go -C src/backend test ./internal/analysis/` | PASS: 68 top-level tests and 37 subtests, 0 failed, 0 skipped; also PASS with `-shuffle=on` and `-count=3` |
| Unit regression | `go -C src/backend test ./...` | PASS: 95 top-level tests (27 Phase 01 + 68 Phase 02) and 62 subtests, 0 failed, 0 skipped; no Docker |
| Integration + acceptance + functional regression | `go -C src/backend test -tags=integration ./...` | PASS: 109 top-level tests (95 above + 14 Phase 01 integration/functional, including the black-box test) and 69 subtests, 0 failed, 0 skipped |
| Race detector | `go test -race ./...` in the `golang:1.27.1` Linux container | PASS for every package |
| Static checks | `gofmt -l src/backend`, `go vet ./...`, `go vet -tags=integration ./...`, `go mod verify`, `go build ./...` | Clean |
| Coverage (gap finding only) | `go test -cover ./internal/analysis/` | 98.1% of statements |
| Duration | `go test -run '^$' -bench BenchmarkAnalyze -benchtime=50x ./internal/analysis/` | 4.3 and 5.8 ms per analysis of the 4,032 readings in two runs (4.0 MB, 32k allocations). Evidence only, not a scale claim |

**Test inventory (`internal/analysis`):**

- **Robust statistics:** median odd/even/order/no mutation; MAD normal/zero/small; deviation positive/negative/stable; zero-MAD floor; zero median; clamp.
- **Baseline:**
  - same hour of the previous 7 days only; not evaluated until the eighth day;
  - **no future leakage** (prefix equality and altered-future equality);
  - frozen baseline under a 12-day shift; flagged readings skipped, then the window resumes;
  - guarded ratio (zero or negative denominator, underflow).
- **Signals:**
  - the dual gate rejects a change with a large Z but only 1% relative deviation;
  - consumption rise and drop, voltage, current, PF, and ratio-only signals with direction and kind;
  - signal evidence fields; an invalid derived ratio produces evidence, not a crash.
- **Episodes:** isolated spike not reportable; sustained shift boundaries and duration; 3 h gap bridged; 4 h gap splits; kinds never merge; recovery observed, absent, and interrupted.
- **Events:** role table (UNKNOWN, unrecognized, DATA_QUALITY, outage vs rise); window and per-meter lookup; offsets and verbatim description; closest explanation.
- **Classification, severity, confidence, action:**
  - decision table (10 cases);
  - severity semantics (why each level follows from the evidence);
  - confidence bounded, finite and deterministic over a grid of extreme inputs;
  - monotonic in evidence; component semantics (alignment and recovery; UNKNOWN; incompatible event; DATA_QUALITY corroboration; ongoing vs recovered);
  - action mapping.
- **Priority:** severity first; REAL before DATA_QUALITY at equal severity; type order; confidence, then evidence, then deterministic tie-break; independent of input order.
- **Synthetic acceptance (fictional meters):**
  - A stable → none;
  - B +80% with current and PF, no event → REAL/HIGH;
  - C same shift + OPERATIONAL_CHANGE → EXPLAINABLE/MEDIUM;
  - D −80% for 10 h + SCHEDULED_OUTAGE + recovery → FALSE_POSITIVE/LOW; D without the event → REAL; outage without recovery → EXPLAINABLE;
  - E inconsistency every third hour → DATA_QUALITY/HIGH, with and without the event; a DATA_QUALITY event alone → none;
  - F single 30% spike → not promoted;
  - UNKNOWN or unrecognized event does not suppress; distant event not correlated; short uncorroborated shift not overclassified; a consumption shift contradicted by flat electrical metrics is not corroborated (REAL/MEDIUM, not HIGH);
  - mixed-fleet priority and meter statuses; evidence completeness; reason built from evidence.
- **Engine:** deterministic across repeats and 5 shuffles; caller slices not modified; invalid input rejected (10 cases incl. off-interval timestamps); events for unknown meters ignored; cancellation; configuration validation.
- **Supplied-data acceptance (parser → engine):** the four scenarios with evidence; priority order; control meters; event effect; perturbation; ±20% threshold stability; determinism under shuffle.

## Acceptance Results On The Supplied Dataset

| Priority | Meter | Type / severity | Confidence | Evidence (from the engine output) |
| --- | --- | --- | --- | --- |
| 1 | M-109 | `REAL_ANOMALY` / `HIGH` | 0.93 | Consumption +110% (median) for 58 h from 2026-09-12 14:00 (5,381 vs 2,563 kWh); current +111% (up) and power factor −22% corroborate; voltage and ratio change in only 1 reading each (not corroborating); `UNKNOWN` event kept as context; no recovery; action `INVESTIGATE_METER_AND_INSTALLATION` |
| 2 | M-112 | `DATA_QUALITY` / `HIGH` | 0.92 | 16 inconsistent readings over 46 h from 2026-09-13 00:00. Voltage and current affected in all 16, power factor in 12, ratio in 9. Consumption never deviates (max 9%, +1.6% over the episode). `DATA_QUALITY` event corroborates. Action `VALIDATE_MEASUREMENT_OR_SENSOR` |
| 3 | M-104 | `EXPLAINABLE_ANOMALY` / `MEDIUM` | 0.73 | Consumption +47% for 96 h from 2026-09-11 00:00 with current +47%. `OPERATIONAL_CHANGE` at the onset (offset 0) explains it; still shifted at the end. Action `VALIDATE_OPERATIONAL_CHANGE` |
| 4 | M-106 | `FALSE_POSITIVE` / `LOW` | 0.76 | Consumption −80% for 12 h from 2026-09-08 00:00 with current −80%. `SCHEDULED_OUTAGE` at the onset. Recovered from 12:00 (post-episode median +1.7%). Action `NO_ESCALATION_MONITOR` |

**False-positive control:** exactly four reportable findings. The eight meters without events have zero flagged readings, so there was no additional finding to investigate. Computed statuses: M-109 CRITICAL, M-112 and M-104 ALERT, all others OK.

## Code Review (actual diff, `code-review` skill)

- **Dataset literals:** no meter ID, dataset timestamp, or event description appears in production code (`grep` of non-test Go files). IDs appear only in `dataset_acceptance_test.go` and documentation. Descriptions are copied into evidence and never inspected.
- **Leakage and contamination:** baselines use only earlier, non-flagged readings; tested.
- **MAD and division:** MAD is floored; zero medians and non-positive ratio denominators make a metric not evaluated. Confidence components are clamped, and NaN becomes 0 (tested).
- **Over-promotion:** 1–2 reading spikes and short uncorroborated unexplained shifts are not reported.
- **Priority:** by severity, type, confidence and evidence; the meter ID is only the last tie-breaker.
- **Confidence vs severity:** computed independently (the FALSE_POSITIVE is LOW severity with 0.76 confidence).
- **Events:**
  - UNKNOWN and unrecognized types never explain;
  - DATA_QUALITY never creates a finding (tested);
  - events for meters without readings are ignored.
- **Dependencies:** standard library only; no concurrency, clock, randomness or LLM code. The configuration is a value copied into the engine; package-level tables are arrays or functions, not mutable maps.
- **Review fixes applied:**
  - the load ratio no longer counts as corroborating a consumption change (a ratio shift means current did not follow), with a new test; supplied-data outcome unchanged;
  - stale deviations cleared on unevaluated readings;
  - singular/plural reason text;
  - hourly-interval validation at the boundary;
  - an unused accessor removed;
  - stable-only Z statistics corrected in the documentation.
- **Scope:** no migration, table, API, sqlc, frontend, LLM, broker or new dependency. `go.mod` is unchanged.

## Phase Exit Gate

| Check | Result |
| --- | --- |
| Build, format, vet, module verification | PASS (commands above) |
| Unit, analytics acceptance, integration, black-box regression | PASS (counts above) |
| Race detector | PASS (Linux container) |
| Database validation | N/A for new work: no schema change. Phase 01 database tests re-run and pass |
| Functional/E2E (Playwright) | N/A: no frontend. The Phase 02 functional boundary (dataset → full pipeline → ordered findings with evidence) is automated by the supplied-data acceptance test through the Phase 01 parser |
| Security and secrets | No secrets or configuration added; the engine performs no I/O |
| Design validation | N/A: no UI |
| Manual acceptance | The engine's output on the supplied data was inspected (findings, metric evidence, meter summaries) and matches the table above |
| Documentation | `docs/ai/anomaly-analysis.md` (rewritten for the implementation), `docs/ai/explainability.md` §1, decision register, traceability, roadmap, project context, architecture, ADR-002 layout note, testing strategy, README status |
| Traceability | FR-DET-001…007, FR-AI-001/002 (engine part), BR-01–BR-08, acceptance rows updated with executed evidence |

## Known Limitations And Deferred Work

- Engine limitations (no re-baselining after a validated change; first 7 days not evaluated; hour-of-day only; hourly data assumed; single-metric electrical drift not reported; zero-baseline hours; one calibration dataset): `docs/ai/anomaly-analysis.md` §17.
- **Phase 03:** persisted analysis runs and anomalies (with configuration provenance), orchestration, API exposure of findings, meter status and consumption windows (OD-06, OD-07, OD-10 exposure), sqlc.
- **Phase 05:** explanation providers consuming the evidence (the Phase 02 reason is deterministic text only).
- The original ±20% test covered one parameter at a time. The independent audit below adds coherent relative-threshold changes and corrects the unsupported gap-stability claim.

## Independent Analytics Audit (2026-09-24)

Scope: independently audit and repair Phase 02 only, without committing, pushing, configuring a remote or starting Phase 03. Final audit status is recorded after the validation evidence below.

### Entry Gate And Git State

- Initial `git status`, `git status --short`, `git diff --stat`, `git diff`, `git diff --cached`, and `git log --oneline --decorate -n 10` were inspected before edits. Nothing staged; the analytics directory was untracked. All existing changes were preserved.
- HEAD is `a81f38e`, not the request's `a1f1542`. The reflog records a history rewrite removing attribution trailers. Comparing their trees shows only two Phase 00 checkpoint-reference corrections in the Phase 01 document; no production-code difference. Phase 01 is legitimately the baseline, and Phase 02 remains uncommitted.
- Initial changes classified: all nine non-test files under `internal/analysis` are Phase 02 production; its ten original test files are Phase 02 tests. Twelve modified context/analytics-skill/README/ADR/AI/architecture/phase/product/testing documents are Phase 02 documentation. `AGENTS.md`, `CLAUDE.md`, and the phase-governance skill contain pre-existing commit-policy changes, unrelated to analytics and preserved. No suspicious product implementation or tracked generated artifacts found. The audit adds one test file; scratch diagnostics, JSON and coverage stay ignored under `tmp/`.
- Phase 01 entry build and integration-tag vet passed. The first integration attempt was blocked by sandbox Docker-pipe permission; the same command with Docker access passed every package, including real PostgreSQL and black-box tests. This was an environment restriction, not an accepted-code regression.
- Requirements: FR-DET-001–007, engine portion of FR-AI-001/002, BR-01–08 and ER-01/02. ADR-001/002/004/006/008 govern engine boundaries and evidence; ADR-003/007 delimit deferred persistence/orchestration. Relevant skills, testing strategy and DoD were read.

### Findings And Fixes

| Severity | Reproduced issue | Resolution / evidence |
| --- | --- | --- |
| Important | Six normal observations separated by missing/unevaluated hours incorrectly established outage recovery and `FALSE_POSITIVE` | Recovery now stops at the first missing interval or unevaluated reading. All three failing continuity scenarios now stay explainable without claimed recovery |
| Important | A changed measured variable and its derived ratio counted twice toward multivariate confidence | Ratio remains consistency evidence but no longer contributes an independent confidence vote; synthetic halved current scores 1/3, not 2/3 |
| Important | Changed voltage/PF could confirm a load increase with flat current, escalating contradictory evidence to HIGH | Ancillary support now requires current to follow consumption; the reproducer remains uncorroborated REAL/MEDIUM |
| Important documentation | Claimed ±20% stability included the episode gap, but the test omitted it | Explicitly tested: 2.4 h drops the intermittent finding, 3.6 h preserves it. Documented the limitation; no retuning to acceptance IDs |
| Minor | Overflow of V×I×PF produced an infinite denominator and a misleading available zero ratio | Non-finite denominator makes the ratio unavailable; regression test passed |
| Minor documentation | README still called Phase 02 Planned; three normal hours were described incorrectly; single-variable drift, confidence monotonicity and cold-start claims were too broad | Owning documents corrected; exact semantics in `docs/ai/anomaly-analysis.md` |
| Minor documentation | Earlier stable robust-Z maximum 17–48 did not match the implemented spread floor | Independently reproduced maxima C/V/I/PF/ratio = 22.85/12.96/24.39/17.54/29.41; corrected owning document |

No unresolved critical finding. Four acceptance rows alone were not used as proof: fixes first failed generalized regression tests, then passed them with the production repairs.

### Reproduced Numerical Evidence

A separate JavaScript scratch calculation parsed only the source readings/events, computed chronological seven-eligible-sample medians/MADs, dual gates and exclusions independently of Go, and checked every exported signal baseline/Z and every finding's metric and consumption aggregates against the engine. It did not read an evaluator file. Source SHA-256 hashes remain `01c953c2daa6503f9696a46096f34c635d67b7394054ad3f628a0230393b162f` (readings) and `750d42f11c2c409af7094153fd07071d3aa3b5a8d68bb418c9fcaf7cbb0c9932` (events).

| Priority | Meter | Type / severity | Confidence | Start (source wall clock) / span | Independent evidence | Action |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | M-109 | REAL_ANOMALY / HIGH | 0.933333 | 2026-09-12 14:00 / 58 h | 5,380.80 observed vs 2,562.95 baseline kWh; median consumption +109.8306%, current +110.6645%, PF −21.5385%; 58 flagged hours; UNKNOWN at onset is context; no recovery | Investigate meter and installation |
| 2 | M-112 | DATA_QUALITY / HIGH | 0.917295 | 2026-09-13 00:00 / 46 h | 16 inconsistent readings; 437.93 vs 430.84 kWh (+1.6456% total); consumption max deviation 8.9883%; V/I/PF/ratio trigger in 16/16/12/9 readings; DATA_QUALITY at onset corroborates | Validate measurement or sensor |
| 3 | M-104 | EXPLAINABLE_ANOMALY / MEDIUM | 0.728401 | 2026-09-11 00:00 / 96 h | 6,858.86 vs 4,664.00 kWh; median consumption +47.3469%, current +47.2987%; OPERATIONAL_CHANGE at onset explains; no recovery | Validate operational change |
| 4 | M-106 | FALSE_POSITIVE / LOW | 0.756361 | 2026-09-08 00:00 / 12 h | 126.40 vs 626.30 kWh; median consumption −79.6895%, current −79.8419%; SCHEDULED_OUTAGE at onset; six consecutive recovery readings from 12:00, median +1.7176% | No escalation; monitor |

All findings are shown; eight meters have zero findings and zero flagged readings. All 12 have 336 readings and 168 evaluated readings. M-109 is CRITICAL; M-104/M-112 ALERT; the other nine OK. Source status is not an engine input and is never overwritten.

| Stable meter | Max consumption % | Max voltage % | Max current % | Max PF % | Max ratio % | Max absolute Z |
| --- | --- | --- | --- | --- | --- | --- |
| M-101 | 15.948 | 2.135 | 8.543 | 5.285 | 15.199 | 28.516 |
| M-102 | 15.654 | 1.837 | 5.409 | 4.399 | 17.236 | 12.900 |
| M-103 | 18.391 | 1.575 | 8.658 | 4.419 | 17.349 | 19.753 |
| M-105 | 14.236 | 1.856 | 5.609 | 4.560 | 15.722 | 13.683 |
| M-107 | 18.270 | 2.170 | 13.895 | 4.984 | 24.837 | 29.409 |
| M-108 | 15.188 | 2.035 | 4.475 | 4.797 | 15.517 | 12.956 |
| M-110 | 15.433 | 1.587 | 8.609 | 5.229 | 26.909 | 17.537 |
| M-111 | 14.258 | 1.603 | 6.576 | 5.207 | 20.389 | 18.019 |

The relative thresholds 25/3/20/8/40% exceed these observed maxima. Huge Z alone does not trigger. The opposite dual-gate test uses a 40% change within a noisy historical spread and correctly rejects it.

### Boundary, Generalization And Design Evidence

- Existing tests prove exactly seven prior eligible same-hour observations, day-eight maturity, prefix equality and unchanged earlier evaluations after future alteration. New tests prove meter isolation, a 25-day borderline-flag scenario with exactly one flag/no finding, and a 17-day legitimate explained shift that remains compared with pre-change behavior. Zero/insufficient baselines create neither findings nor fake evaluated evidence.
- Exact gap tests: 2/3 h merge, 4 h splits. Two flags are not reportable; three are. 11 h is not sustained; 12 h is. Five recovery observations are insufficient; six suffice if uninterrupted. Sparse observations keep elapsed duration, observed sample count and longest consecutive run distinct.
- Event offsets −4/−3/0/+3/+4 h: outside/inside/inside/inside/outside. Multiple events resolve identically under reversal, with nearest then earlier then lexical tie rules. Structured UNKNOWN/unrecognized events do not suppress; DATA_QUALITY alone creates nothing. Descriptions are never parsed.
- All generalized classification scenarios pass. Proportional load/current growth preserves the ratio; a stable-consumption denominator change can produce inconsistency; a consumption rise without following current remains unsupported. Ratio evidence never independently increases confidence.
- Complete result equality under whole-day timestamp shift and fictional IDs includes metrics, signals, reasons, recovery and summaries. Repeated/shuffled readings and events preserve all results and caller inputs. Production source scans found no challenge IDs, challenge dates/descriptions, result cap, whitelist or top-four suppression.
- Priority comparator: severity, type risk, confidence, evidence strength, earlier onset, meter ID. No map-order dependence. Confidence/severity are independent; confidence is bounded and monotonic within fixed context, with intentional steps documented in the owning algorithm document.
- Engine performs no I/O, network, clock access, random generation, concurrency or persistence. Its nine production files use only the standard library. Source sorting is O(N log N); per-reading history is bounded at seven, with fixed metric count; event lookup uses a sorted per-meter index. No full-dataset quadratic rescan or speculative abstraction.
- Only the Phase 01 source migration exists (meters/readings/events). Operational routes only; no product API, sqlc, background runner, frontend, explanation provider or LLM integration.

### Final Executed Validation

Commands run from repository root; JSON logging was used to count real test events.

| Check | Command | Result |
| --- | --- | --- |
| Unit | `go -C src/backend test -json -count=1 ./...` | PASS: 113 top-level + 76 subtests; 0 failed/skipped |
| Shuffle | `go -C src/backend test -json -shuffle=on -count=1 ./...` | PASS: 113 + 76; 0 failed/skipped |
| Three repetitions | `go -C src/backend test -json -count=3 ./...` | PASS: 339 top-level executions + 228 subtest executions; 0 failed/skipped |
| Integration and black-box | `go -C src/backend test -json -tags=integration -count=1 ./...` | PASS: 127 + 83; real PostgreSQL 18.6; 0 failed/skipped |
| Analytics coverage | `go -C src/backend test -count=1 -coverprofile=tmp/audit/coverage.out ./internal/analysis` | PASS: 98.8%; 86 top-level analytics tests. Added meaningful missing-baseline tests; remaining uncovered branches are defensive non-finite/unknown-enum defaults, wording variants, and the in-loop cancellation return (pre-cancel behavior tested) |
| Performance | `go -C src/backend test -run '^$' -bench BenchmarkAnalyze -benchtime=50x -benchmem ./internal/analysis` | PASS: 14.018 ms/op, 4,043,932 B/op, 32,449 allocs/op during concurrent validation. Earlier 4–6 ms not reproduced; no timing guarantee claimed |
| Static | `gofmt -l src/backend`; `go -C src/backend mod tidy`; `go -C src/backend mod verify`; `go -C src/backend vet ./...`; `go -C src/backend vet -tags=integration ./...`; `go -C src/backend build ./...`; `git diff --check` | PASS; no module-file changes; formatting clean |

Race validation: **PASS for both `go test -race -count=1 ./...` and `go test -race -count=1 -tags=integration ./...` on Linux**, including the black-box test and PostgreSQL 18.6; no races reported. The plain Go image initially lacked the Docker CLI used by the existing black-box pause/unpause test; analytics and database packages passed, while that test failed on the missing executable. A temporary image copied the official Docker CLI into `golang:1.27.1`, and the complete rerun passed. No test was skipped or weakened.

The executed temporary-image procedure (PowerShell, repository root) is reproducible with this Dockerfile saved outside tracked product files:

```dockerfile
FROM docker:cli AS cli
FROM golang:1.27.1
COPY --from=cli /usr/local/bin/docker /usr/local/bin/docker
```

```powershell
docker build -t bia-phase02-audit -f tmp/Dockerfile.phase02-audit tmp
$auditRoot = (Get-Location).Path
$auditModCache = go env GOMODCACHE
docker run --rm --mount "type=bind,source=$auditRoot,target=/workspace,readonly" --mount "type=bind,source=$auditModCache,target=/go/pkg/mod" --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal -w /workspace/src/backend bia-phase02-audit sh -c 'go test -race -count=1 ./... && go test -race -count=1 -tags=integration ./...'
```

The actual run used these mounts with resolved absolute paths. Go image digest: `sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244`; Docker CLI source digest: `sha256:018edbc908e08fcc9dbf029c812c34251e9b4719e6f71ca0e5eae2a987d014ca`.

UI/Playwright/design acceptance: N/A because this phase has no frontend. Manual acceptance: all production findings, signal/metric evidence and meter summaries inspected and independently recomputed. Actual diff review includes the untracked analytics files, not only tracked documentation. No source CSV change, product dependency, secret, migration or phase expansion. No commit/push/remote change performed.

**Exit decision: COMPLETE WITH NON-BLOCKING NOTES.** Mandatory Phase 02 behavior is reproduced and the identified production defects are repaired. The non-blocking limitations are the gap sensitivity, frozen baselines after legitimate shifts, cold start (and initial-history contamination risk), single-feature reporting limits and heuristic calibration on one small dataset, as precisely documented in `docs/ai/anomaly-analysis.md` §17. Phase 02 remains Complete with these notes. **READY FOR PHASE 03** means technically ready only: **no next phase is authorized**, and Phase 03 was not started. Recommended checkpoint, not executed: `feat: implement deterministic anomaly analysis engine`.

## Next Phase

None authorized. Phase 03 requires explicit user authorization and starts with its Entry Gate: all of the above, plus the analytics unit tests and acceptance scenarios.

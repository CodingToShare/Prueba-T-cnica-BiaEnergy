# Traceability Matrix

Maps each important requirement to its implementation phase, component, planned test levels, and evidence. Evidence is filled only with tests that actually ran and passed, in the same change that produces them. Phase 01 evidence is in [phase-01-foundation-data.md](../phases/phase-01-foundation-data.md). Phase 02 evidence is in [phase-02-analytics-engine.md](../phases/phase-02-analytics-engine.md).

Independent Phase 02 audit evidence is recorded in that phase document under "Independent Analytics Audit". `audit_test.go` covers recovery continuity (FR-DET-007/BR-03), ratio overflow and independent confidence votes (FR-DET-005/006, BR-02/05), coherent current support (BR-04), baseline feedback and cold start (FR-DET-002/004), exact episode/event boundaries, and status/priority semantics. Extended dataset tests verify complete evidence under a whole-day shift and renaming (ER-01/02), coherent relative-threshold sensitivity, and the explicit gap-sensitivity limitation. Final executed regression: 113 unit tests + 76 subtests; 127 integration/black-box tests + 83 subtests; zero failures or skips. The audit's source-CSV recomputation independently matches all signal baselines/Z and finding aggregates (FR-AI-002/BR-07).

Levels: **U** unit · **I** integration (real PostgreSQL / API) · **F** functional/E2E (Playwright, or API-level acceptance in Phase 03) · **V** visual/manual review · **R** repository/review check.

Status: `Planned` → `In Progress` → `Verified`.

## Product Capabilities

| Requirement | Capability | Phase | Component | Planned levels | Evidence | Status |
| --- | --- | --- | --- | --- | --- | --- |
| FR-DATA-002 | Dataset ingestion (12 meters, 4,032 readings, events; idempotent; source unmodified) | 01 | `cmd/seed`, `internal/ingestion` | U, I, F | U: `parse_test.go`, `report_test.go` (parsing, malformed values, headers, duplicates, unusual values kept). I: `store_integration_test.go` (idempotency, update, atomic failure). Acceptance: `TestAcceptance_SuppliedDataset_LoadsCompletelyAndIdempotently` (12/4032/4, 336 per meter, range, exact values, hashes unchanged). Events compared field by field; re-import rewrites no row (`ctid`/`xmin`). F: `TestBlackBox_MigrateSeedServe_OverRealProcessesAndHTTP` (compiled `migrate` and `seed` processes), `TestSeed_SuppliedDatasetTwice_KeepsExactCounts` (in-process); README lifecycle executed | Verified |
| FR-DATA-001 | Persistence model and constraints | 01, 03 | `database/migrations` | I | Meter, Reading, Event: `TestMigrations_EmptyDatabase_UpDownUp`, `TestMigrations_SchemaMatchesSourceContract`, `TestSchema_EnforcesIntegrityInTheDatabase`. Anomaly and AnalysisRun: Phase 03 | In Progress (source entities verified) |
| FR-DASH-001 | Dashboard summary KPIs | 03, 04 | `internal/dashboard`, dashboard page | I, U (component), F | | Planned |
| FR-DASH-002 | Reach highest-priority anomaly from dashboard | 04 | dashboard page | F | | Planned |
| FR-MTR-001…004 | Meters list, status filter, search, sort | 03, 04 | `internal/meter`, meters page | I, U (component), F | | Planned |
| FR-MTR-005 | Meter detail and readings history with V, I, PF, events, anomaly markers | 03, 04 | readings endpoint, detail page, ECharts | I, U (component), F, V | | Planned |
| FR-ANL-001 | Run AI Analysis | 03, 04 | run orchestration, run action | U, I, F | | Planned |
| FR-ANL-002 | Analysis status/progress stages | 03, 04 | `analysis_runs`, progress UI | U (state machine), I, F | | Planned |
| FR-ANL-003 | Completion summary counts | 03, 04 | run summary | I, U (component) | | Planned |
| FR-ANOM-001 | Anomaly list in priority order (incl. false positives) | 03, 04 | anomalies endpoint and page | I, U (component), F | | Planned |
| FR-INV-001 | Anomaly detail / investigation (7 elements) | 03, 04, 05 | anomaly detail endpoint, investigation page | I, U (component), F, V | | Planned |
| FR-AI-001 | Finding structure (type, severity, confidence, reason, action) | 02, 03 | engine output, API payload | U, I | Engine (Phase 02): `internal/analysis` `Finding`; U: `TestFindings_CarryCompleteEvidence`, `TestAcceptance_SuppliedDataset_ClassifiesTheFourScenariosWithEvidence`. API payload: Phase 03 | In Progress (engine verified) |
| FR-AI-002 / BR-07 | Explanation backed by evidence; grounding | 02, 05 | evidence, `ExplanationProvider` | U, F | Engine evidence (Phase 02): signals, metric evidence, persistence/recovery, related events with roles, rule, confidence breakdown; U: `TestFindings_CarryCompleteEvidence`, `TestReason_IsBuiltFromEvidence`, `TestSignals_SignalEvidenceCarriesTheMeasurement`. Providers and grounding validation: Phase 05 | In Progress (evidence verified) |
| BR-08 | Recommended action coherent with classification | 02, 05 | engine recommendation, templates | U, F | Engine action codes (Phase 02): `TestRecommendedAction_IsCoherentWithClassification`, scenario tests A–E, supplied-data acceptance. Operator wording: Phase 05 | In Progress (engine verified) |
| FR-DET-001…007 | Detection capabilities | 02 | `internal/analysis` | U | U (Phase 02): spikes/abrupt changes `TestScenarioF_IsolatedSpike_IsNotPromoted`, `TestSignals_EachMetricAndDirection`; persistent change `TestScenarioB_…`, `TestBaseline_SustainedShift_StaysComparedWithPreEpisodeBehavior`, `TestEpisodes_*`; outliers (dual gate) `TestSignals_DualGate_…`; hourly patterns (hour-of-day baselines) `TestBaseline_UsesSameHourOfThePreviousSevenDaysOnly`; data quality `TestScenarioE_*`; variable relationships `TestSignals_RelationshipOnlyChange_OnlyTheRatioTriggers`, `TestFeatures_LoadRatioIsRelativeAndGuarded`; explained changes `TestScenarioC_…`, `TestScenarioD_*`, `TestAcceptance_SuppliedDataset_EventCorrelationChangesTheClassification` | Verified |
| FR-AUTH-001 | Simple login entry | 03, 04 | minimal auth | I, F | | Planned |
| FR-UX-001 | Responsive SaaS product UX in canonical visual language | 04, 05, 06 | whole frontend | U (component), F, V | | Planned |
| FR-DEL-001 | Deliverables (repo, frontend, backend, demo) | 06 | repository, demo script | F, V | | Planned |
| FR-DEL-002 | Reproducible startup / demo reset | 01 (first steps), 06 | `compose.yaml`, README commands, entry point (OD-19) | I, F, V (clean clone) | Phase 01: clean-database run (reset → compose healthy → migrate → import ×2 → API ready), README commands verified in PowerShell and Bash; full README lifecycle re-executed in the Phase 01 audit. Single entry point and demo reset: Phase 06 | In Progress |
| FR-API-001 | Minimum API under `/api/v1` with OpenAPI | 03 | handlers, OpenAPI | I, F (API-level) | | Planned |
| FR-TECH-001 | Go backend | 01 | `src/backend` (Go 1.27.1) | R (build) | `go build ./...`, `go vet ./...`, gofmt clean | Verified |
| FR-DATA-003 / BR-09 | Evaluator file never used | 00–06 | whole repository | R | Phase 00 scan: only prohibitions present. Phase 02: calibration used only `readings.csv`/`events.csv`; `grep` of `src/` finds no reference | Planned (ongoing) |

## Operational Endpoints (Technical Decision TD-10)

| Capability | Phase | Component | Evidence | Status |
| --- | --- | --- | --- | --- |
| `/healthz` liveness, `/readyz` PostgreSQL readiness, graceful shutdown | 01 | `internal/platform/health`, `cmd/api` | U: `health_test.go` (200, 503 without leaking errors). F (black-box): `TestBlackBox_MigrateSeedServe_OverRealProcessesAndHTTP` (separate API process, real HTTP, 503 while PostgreSQL is paused, recovery, SIGTERM → exit 0 on Linux, no password in output). In-process: `TestAPI_HealthReadinessAndGracefulShutdown`, `TestAPI_DatabaseUnreachableAtStartup_FailsFast` | Verified |

## Analytics Acceptance

| Rule / scenario | Expected | Phase | Planned levels | Evidence | Status |
| --- | --- | --- | --- | --- | --- |
| BR-03 / M-104 | `EXPLAINABLE_ANOMALY` / `MEDIUM`; compatible operational event correlated | 02 | U (acceptance), I/F via API in 03 | `TestAcceptance_SuppliedDataset_ClassifiesTheFourScenariosWithEvidence` (M-104: +47% for 96 h, `OPERATIONAL_CHANGE` at onset with role EXPLAINS, still shifted, confidence 0.73, action `VALIDATE_OPERATIONAL_CHANGE`); synthetic `TestScenarioC_…` | Verified (engine) |
| BR-03 / M-106 | `FALSE_POSITIVE` / `LOW`; deviation present but explained; never `REAL_ANOMALY` | 02 | U (acceptance + event-effect test) | M-106 in the same test (−80% for 12 h, `SCHEDULED_OUTAGE` EXPLAINS, recovered from 12:00, confidence 0.76); event effect `TestAcceptance_SuppliedDataset_EventCorrelationChangesTheClassification`; synthetic `TestScenarioD_*` | Verified (engine) |
| BR-04 / M-109 detection | `REAL_ANOMALY` / `HIGH`; deviation, persistence, electrical changes, no explaining event | 02 | U (acceptance), F (API 03, UI 04–05) | M-109 in the same test (+110% for 58 h, current +111% and power factor −22% corroborate, `UNKNOWN` is context only, confidence 0.93, action `INVESTIGATE_METER_AND_INSTALLATION`); synthetic `TestScenarioB_…`, `TestUnknownOrUnrecognizedEvent_DoesNotSuppressAStrongAnomaly`. API/UI: Phases 03–05 | In Progress (engine verified) |
| BR-06 / M-109 prioritization | Ranked first among findings | 02, 03 | U, I, F | Engine: `TestAcceptance_SuppliedDataset_PrioritizesTheRealAnomalyFirst` (M-109, M-112, M-104, M-106), `TestPriority_*`, `TestMixedDataset_PrioritizesByEvidence`. API order: Phase 03 | In Progress (engine verified) |
| BR-05 / M-112 | `DATA_QUALITY` / `HIGH`; electrical inconsistency with stable consumption | 02 | U (acceptance) | M-112 in the same test (16 inconsistent readings over 46 h, consumption never deviates, voltage/current/PF/ratio affected, `DATA_QUALITY` event CORROBORATES, confidence 0.92); synthetic `TestScenarioE_*` (with and without event; event alone creates nothing) | Verified (engine) |
| BR-01, BR-02 | Every finding has one type, severity, confidence, reason, action | 02 | U | `TestFindings_CarryCompleteEvidence`, `TestClassify_DecisionRules`, `TestSeverity_FollowsFromEvidenceNotOnlyFromType`, `TestConfidence_*` | Verified |
| ER-01, ER-02 | No dataset special-casing; perturbation test stable | 02–06 | U (perturbation), R | Phase 02: `TestAcceptance_SuppliedDataset_PerturbationKeepsTheResult` (renamed meters, +37 days 5 h), `TestAcceptance_SuppliedDataset_IsStableUnderThresholdChanges` (±20%), control meters quiet; production-code `grep` finds no meter ID, dataset timestamp or description literal | In Progress (verified for Phase 02) |

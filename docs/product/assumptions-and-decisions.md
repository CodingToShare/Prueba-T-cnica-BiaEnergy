# Assumptions And Decisions

This register keeps `TECHNICAL DECISION`, `ASSUMPTION`, and `OPEN DECISION` separate from requirements (`functional-requirements.md`) and business rules (`business-rules.md`). An entry changes category only through an explicit, recorded decision.

## Technical Decisions (Accepted)

| ID | Decision | Reference |
| --- | --- | --- |
| TD-01 | Pragmatic modular monolith: one Go API, one Next.js frontend, one PostgreSQL database. | ADR-001 |
| TD-02 | Go backend with chi, pgx, sqlc, goose, slog; idiomatic packages by capability. | ADR-002 |
| TD-03 | Public API versioned under `/api/v1` and described with OpenAPI 3. | Architecture |
| TD-04 | PostgreSQL is the source of truth; JSONB only for flexible structured evidence. | ADR-003 |
| TD-05 | Anomaly detection, classification, severity, confidence, and evidence are deterministic and reproducible. | ADR-004 |
| TD-06 | The challenge data model is extended with `AnalysisRun` and `AnomalyEvidence`. | Architecture |
| TD-07 | Meter `source_status` (from CSV) is distinct from analytically derived `computed_status`. | Architecture |
| TD-08 | Next.js + React + TypeScript + Tailwind + shadcn/ui + TanStack Query + ECharts frontend. | ADR-005 |
| TD-09 | Generative AI is an optional explanation layer behind `ExplanationProvider`; deterministic provider is default; Ollama preferred. | ADR-006 |
| TD-10 | Operational endpoints: health (liveness), readiness (database reachable), Prometheus-compatible metrics; structured logs with `slog`. | Architecture |
| TD-11 | Analysis runs are persisted and executed by a lightweight in-process background goroutine; no broker. | ADR-007 |
| TD-12 | Source observation times are timezone-naive `timestamp` values holding the source wall clock; system-generated instants use `timestamptz`. | ADR-008 (amends ADR-003) |
| TD-13 | Dependency versions are selected and pinned when first introduced, not during Phase 00. | Roadmap |
| TD-14 | Agent governance files (`AGENTS.md`, `CLAUDE.md`, `.github/copilot-instructions.md`, `.agents/`) are tracked; per-user tool state is ignored. | `AGENTS.md` §14 |
| TD-15 | `docs/design/reference/design-system-v3.html` is the canonical visual reference (Bia Energy edition of v3, illustrative values only; changes only by explicit recorded design decision); `docs/design/design-system.md` is the product implementation contract. | Design system |
| TD-16 | Testing is continuous: every phase adds its own unit/integration/functional evidence and passes a Phase Entry Gate and Phase Exit Gate; Phase 06 is hardening and full regression, not first-time testing. | Roadmap, testing strategy |
| TD-17 | Measurements are `NUMERIC` in PostgreSQL (exact decimal persistence and aggregation) and `float64` in Go; pgx's shortest round-trip encoding reproduces source values exactly. | Data model |
| TD-18 | sqlc is introduced with the Phase 03 product read queries; the Phase 01 bulk ingestion uses explicit parameterized pgx batch statements, which report inserted/updated/unchanged rows. | ADR-002, data model |
| TD-19 | goose runs as a library through `cmd/migrate` (version pinned in `go.mod`); no globally installed migration tool. | ADR-002 |
| TD-20 | Go integration tests live beside their package, behind the `integration` build tag, using testcontainers-go with the environment-validated `postgres:18.6-alpine` image; backend black-box tests live in `src/backend/functional`; `go test ./...` stays Docker-free. | Testing strategy |
| TD-21 | Go module path is `bia-energy.local/backend`, a deliberately non-public placeholder because no canonical remote exists; rename when a repository URL is defined. | Phase 01 |

## Assumptions

| ID | Assumption | Impact | Status |
| --- | --- | --- | --- |
| AS-01 | Primary users are energy/operations analysts monitoring a small meter fleet. | Copy and workflows target an operator deciding what to investigate. No role model is derived. | Proposed |
| AS-02 | Source timestamps have no zone and represent one consistent local time. | Superseded: they are stored timezone-naive, with no zone assumed (ADR-008). | Resolved by ADR-008 (Phase 01) |
| AS-03 | Each reading is the energy consumed during a one-hour interval starting at `timestamp`. | Daily and period totals are sums of hourly readings. | Proposed |
| AS-04 | The dataset is complete and regular (one reading per meter per hour). | Verified in Phase 01: 12 meters × 336 hourly readings, 0 gaps, 0 irregular steps, 0 duplicates. | Confirmed by evidence (Phase 01) |
| AS-05 | `event_timestamp` marks the start of an event; there is no end-time column. | Event influence windows need a decision (OD-09). Phase 01 confirms the file has no end-time column and stores descriptions verbatim. | Proposed |
| AS-06 | Single tenant, single site, small fleet; no multi-tenancy. | No tenant scoping in schema or API. | Proposed |
| AS-07 | The evaluation demo runs locally. | Local-first delivery via Docker Compose. | Proposed |

## Open Decisions

Each is resolved in the named phase, recorded here with its rationale, and reflected in the owning document.

| ID | Open decision | Notes | Resolve in |
| --- | --- | --- | --- |
| OD-01 | Detection thresholds (robust Z-score, percentage deviation, persistence length). | Calibrate against the supplied data; store in one configuration location. | Phase 02 |
| OD-02 | Baseline reference window and contamination handling. | 14 days allows hour-of-day profiles but very few samples per weekday-hour; the baseline must not be distorted by the anomaly itself. | Phase 02 |
| OD-03 | Confidence formula and signal weighting. | Signals are fixed by ADR-004; weights are not. | Phase 02 |
| OD-04 | Severity computation. | Must depend on magnitude, persistence, corroboration, and classification — not on classification alone. | Phase 02 |
| OD-05 | Priority ranking function. | Must rank a high-severity real anomaly first (BR-06); ties between equal severities need a documented order. | Phase 02 |
| OD-06 | Windows for "current consumption" and "variation" in list, detail, and KPIs. | Challenge figures (e.g., M-109 2,180 kWh vs baseline ~1,070 kWh) are illustrative; the product computes and states its own window. | Phase 02–03 |
| OD-07 | Definition of the aggregated "AI confidence" KPI. | For example, mean confidence of escalated findings. | Phase 03 |
| OD-08 | Event-type semantics: which event types can explain which deviation directions; treatment of `UNKNOWN` ("no operational event reported") and `DATA_QUALITY` events. | Events corroborate or explain; data-derived signals must still exist. An `UNKNOWN` event is not an explanation. | Phase 02 |
| OD-09 | Event influence window. | Durations exist only in free-text descriptions; options: parse cautiously, infer the window from data aligned to the event start, or a documented default. | Phase 02 |
| OD-10 | Mapping from findings to meter `computed_status` (OK / Alert / Critical). | Challenge example shows a high-severity data-quality meter as Alert, not Critical. | Phase 02–03 |
| OD-11 | Whether an `INVESTIGATE`/`UNKNOWN` classification is needed for ambiguous findings. | Not a requirement; add only if calibration shows real ambiguity. | Phase 02 |
| OD-12 | Login mechanism. | Must stay simple (e.g., single configured demo credential with a session cookie); no OAuth/RBAC. | Phase 03–04 |
| OD-13 | UI language (Spanish vs English). | Challenge is in Spanish; API enum values remain English constants. | Phase 04 |
| OD-14 | Concurrent analysis runs and which run is "current". | For example, reject a new run while one is active; current = latest completed. | Phase 03 |
| OD-15 | Exact dependency versions. | Pinned at introduction (TD-13). | Phase 01+ |
| OD-16 | Whether the demo enables Ollama and which local model. | The product must be complete without it. | Phase 05 |
| OD-17 | Granularity of a finding: one per meter per run, or one per detected episode. | The challenge shows one row per meter. | Phase 02 |
| OD-18 | Whether readings flagged as data-quality problems are excluded from baselines and consumption KPIs. | Affects baseline robustness and KPI honesty. | Phase 02 |
| OD-19 | Entry point for reproducible dev/seed/test/demo-reset (e.g., Make targets vs cross-platform scripts vs Compose-only). | Must work on the evaluator's OS; `make` is not available by default on Windows. Phase 01 uses plain cross-platform `docker compose` and `go run` commands (README). The final single entry point is decided in Phase 06. | Phase 06 |

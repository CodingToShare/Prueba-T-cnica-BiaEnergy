# Project Context (Agent Bootstrap)

Concise context for coding agents. Canonical rules live in `AGENTS.md`; canonical facts live in the documents linked below. If this file disagrees with them, they win.

## What And Why

**Bia Energy Management Platform** — an MVP built for a 3-day Full Stack + Data + AI technical challenge. It turns hourly electrical-meter data into operational decisions: which readings deviate, whether each deviation is real, explainable, a false positive, or a data-quality problem, which one to investigate first, why, with what evidence, and what to do.

Success cycle: `DATA → ANALYSIS → ANOMALY → EXPLANATION → PRIORITIZATION → ACTION`.
Demo flow (5–10 minutes): `Login → Dashboard → Meters → Meter Detail → Run AI Analysis → AI Anomalies → Investigation → Recommended Action`.

## Current Phase

Phases 00, 01 and 02 are Complete. Phase 01 delivered the Go module (`cmd/api`, `cmd/migrate`, `cmd/seed`), the PostgreSQL schema for meters, readings and events, and the verified idempotent import of the supplied dataset. Phase 02 delivered the pure deterministic anomaly engine `internal/analysis` (`Engine.Analyze`), verified on the four acceptance scenarios; nothing calls it yet. **No product API, persisted analysis runs or frontend exist yet.** Phase 03 onward requires explicit user authorization. Commands: `README.md`; data model: `docs/architecture/data-model.md`.

## How Agents Work Here

```text
Authorized phase → read context → Phase Entry Gate (baseline green?) → implement scope only
→ tests with the code → focused + regression validation → diff review → manual/design acceptance
→ docs + traceability → Definition of Done → Complete only with evidence → STOP
```

Testing is continuous: each phase adds its own unit/integration/functional evidence; Phase 06 is hardening and full regression, not first-time testing. A broken baseline is reported as "Blocked by baseline regression". Details: `AGENTS.md` §2, `docs/phases/roadmap.md`.

## Visual Language

Canonical visual reference: `docs/design/reference/design-system-v3.html` (navy `#1E3A5F` primary, Plus Jakarta Sans, light surfaces, restrained shadows). It is the Bia Energy edition of design system v3: dashboard, investigation, states, and semantic badges with illustrative values (not requirements, not engine output). The product contract with Energy semantics (normal, informational, warning, critical, data-quality) is `docs/design/design-system.md`.

## Approved Architecture And Stack

Pragmatic modular monolith (ADR-001).

- Backend: Go, chi, pgx, sqlc, goose, slog (ADR-002).
- Database: PostgreSQL; JSONB only for flexible evidence (ADR-003).
- Frontend: Next.js, React, TypeScript, Tailwind CSS, shadcn/ui, TanStack Query, Apache ECharts (ADR-005).
- Analytics: deterministic robust-statistics engine in Go (ADR-004).
- Generative AI: optional `ExplanationProvider`; Ollama preferred; product works without it (ADR-006).
- Analysis runs: persisted `AnalysisRun` + lightweight in-process Go background execution; no broker (ADR-007).
- Testing: Go `testing` (+ testify where valuable), testcontainers-go, Vitest + Testing Library, Playwright.
- Delivery: Docker, Docker Compose, GitHub Actions, OpenAPI 3, health/readiness, Prometheus-compatible metrics.
- Dependency versions are pinned when first introduced, not before.

## Critical Dataset Facts

- `data/input/readings.csv`: 12 meters (M-101…M-112), 14 days (2026-09-01 00:00 → 2026-09-14 23:00), hourly, 4,032 rows. Columns: `meter_id, timestamp, consumption_kwh, voltage_v, current_a, power_factor, status`.
- `data/input/events.csv`: known events. Columns: `meter_id, event_timestamp, event_type, description`. Event types seen: `OPERATIONAL_CHANGE`, `SCHEDULED_OUTAGE`, `UNKNOWN`, `DATA_QUALITY`. Timestamps are minute-precision (different format from readings). Event duration, when present, appears only in free-text descriptions.
- The source `status` column is `OK` for all 4,032 rows, including the electrically inconsistent ones (verified in Phase 01); it is `source_status`, not the analytical health.
- Source timestamps are timezone-naive and stored as `timestamp without time zone` (ADR-008).
- `expected_results.csv` is **evaluator-only**. Never create, search for, or use it.
- The CSVs are in `data/input/` (byte-identical to the supplied files; hashes in `data/input/README.md`). Never modify or regenerate them.

## Acceptance Scenarios (tests, never code branches)

| Meter | Observed behavior | Expected semantics |
| --- | --- | --- |
| M-104 | Persistent consumption increase coinciding with a new production line event | `EXPLAINABLE_ANOMALY` / `MEDIUM` |
| M-106 | Consumption drop during a scheduled maintenance outage | `FALSE_POSITIVE` / `LOW` |
| M-109 | >~100% persistent increase, electrical changes, no explanatory event | `REAL_ANOMALY` / `HIGH`, prioritized first |
| M-112 | Stable consumption, inconsistent electrical readings | `DATA_QUALITY` / `HIGH` |

The engine must reach these through generalized analysis. See `docs/ai/anomaly-analysis.md`.

## Deterministic AI Principle

Detection, classification, severity, confidence, and evidence are computed deterministically. An LLM may only turn structured evidence into operator-friendly wording and must be optional, grounded, and replaceable by the deterministic provider.

## Important Non-Goals

Microservices, brokers, Redis, Kubernetes, Elasticsearch, TimescaleDB, event sourcing, full CQRS, RAG, vector databases, agent frameworks, a Python ML service, complex auth, speculative abstractions. Full list: `docs/product/out-of-scope.md`.

## Repository Map

```text
.agents/            agent context and skills (shared governance)
.github/            Copilot entry point (CI workflows come in Phase 06)
data/input/         source CSVs (placed by a person; never modified)
database/           migrations/ (goose, since Phase 01) and queries/ (sqlc, from Phase 03)
docs/adr/           accepted architecture decisions
docs/ai/            PRODUCT analytics and explainability design
docs/architecture/  system architecture
docs/design/        design contract + reference/design-system-v3.html (canonical visual sample)
docs/performance/   data query strategy
docs/phases/        roadmap and phase contracts/evidence
docs/product/       requirements, rules, decisions, scope, traceability
docs/quality/       Definition of Done
docs/testing/       testing strategy
src/backend/        Go module: cmd/api, cmd/migrate, cmd/seed, internal/* (since Phase 01)
src/frontend/       Next.js app (from Phase 04)
tests/e2e/          Playwright journeys
```

## Where Canonical Rules Live

Engineering rules: `AGENTS.md`. Requirements and rules: `docs/product/`. Decisions: `docs/adr/` and `docs/product/assumptions-and-decisions.md`. Analytics: `docs/ai/`. Completion: `docs/quality/definition-of-done.md`. Phases: `docs/phases/`.

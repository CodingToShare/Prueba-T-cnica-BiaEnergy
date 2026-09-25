# Delivery Roadmap

Sequential, phase-gated delivery for a three-day challenge. Procedures are in the `phase-governance` skill; completion criteria are in `docs/quality/definition-of-done.md`.

| Phase | Name | Status | Primary outcome |
| --- | --- | --- | --- |
| 00 | Repository + AI Agent + Engineering Governance Foundation | Complete (after corrective validation) | Clean, coherent, agent-ready workspace with canonical visual and test governance |
| 01 | Runtime Foundation + PostgreSQL + Dataset Ingestion | Complete | Go runtime, PostgreSQL schema, migrations, and verified idempotent dataset ingestion |
| 02 | Analytics / Anomaly Engine | Complete (independent audit; non-blocking notes) | Deterministic, evidence-producing engine passing the four acceptance scenarios |
| 03 | Backend API + Analysis Orchestration | Planned / Not Started | Versioned, documented API with persisted analysis runs and progress |
| 04 | Frontend Product Experience | Planned / Not Started | Responsive SaaS-quality product UI matching the canonical visual language |
| 05 | AI Investigation / Explainability Integration | Planned / Not Started | Grounded deterministic + optional generative explanation and investigation experience |
| 06 | Quality + Observability + Delivery + Demo | Planned / Not Started | Complete regression, observability, reproducible delivery, CI, and polished demo |

## Dependencies

Sequential: each phase requires the previous one Complete. Phase 04 may begin against Phase 03 only when Phase 03 is Complete, or when the user explicitly authorizes an agreed parallelization after the API contracts are stable. Authorization is never inferred.

## Governance Gates

| Gate | Rule |
| --- | --- |
| **Phase authorization** | A phase is executed only when the user explicitly authorizes it. |
| **Entry gate** | Previously accepted implementation is validated before new work (see below). |
| **Scope gate** | Work stays inside the authorized phase's scope and non-goals. |
| **Test gate** | Applicable focused tests and regression tests pass. |
| **Design gate** | UI work is validated against `docs/design/reference/design-system-v3.html` and the design contract at ~1440/768/390 px. |
| **Documentation gate** | Phase evidence and traceability are updated in the same change. |
| **Exit gate** | A phase becomes Complete only when objective evidence exists for every applicable check. |
| **Next-phase gate** | Completing one phase never authorizes another. |

Delivery sequence for every phase: Implement → Validate → Review → Document → Complete. Never Implement → Mark Complete → Test Later.

## Phase Entry Gate

Before implementing an authorized phase: confirm the previous phase is Complete; read the governing documents; inspect `git status` and preserve unrelated changes; run the applicable baseline validation; confirm accepted builds/tests still pass; validate previous evidence; review known limitations; confirm the work belongs to the phase. A broken baseline makes the phase **Blocked by baseline regression** until understood; regressions of accepted work are repaired before new behavior is added. Details: `phase-governance` skill.

## Baseline Validation (Evolves With The Product)

Run everything that exists and applies at that point. Concrete commands are recorded by the phase that establishes them; none are prescribed before they exist.

| Before | Baseline to validate |
| --- | --- |
| Phase 01 | Repository and governance validation (structure, legacy scan, document consistency, design reference present and consistent with the design contract) |
| Phase 02 | Backend build and static checks; Phase 01 unit tests; PostgreSQL integration tests; ingestion and data-integrity checks. Commands: `go -C src/backend build ./...`, `go -C src/backend vet -tags=integration ./...`, `go -C src/backend test -tags=integration ./...` (Docker required) |
| Phase 03 | All of the above; analytics unit tests; analytics acceptance scenarios; existing integration tests. Commands: the Phase 02 list plus `go -C src/backend test ./internal/analysis/` (included in both suites; no Docker) |
| Phase 04 | Backend build and tests; API integration tests; OpenAPI/contract checks; frontend bootstrap/build checks once available |
| Phase 05 | Complete backend regression; frontend unit/component regression; Playwright critical smoke flow |
| Phase 06 | Complete existing automated suite; Docker/runtime baseline; known acceptance flow |

## Phase Exit Gate

Applicable checks: build, formatting, linting, static analysis, TypeScript type checking, unit, integration, functional/E2E, database validation, security/secret review, actual git diff review, regression suite, manual acceptance, design validation, documentation, traceability, known limitations, visible evidence. Only applicable checks are required (e.g., no Playwright before a frontend exists), and every N/A has a reason. A phase cannot be Complete with a known blocking failure.

## Regression Rule

Every phase runs focused tests for new or changed behavior **plus** all previously applicable regression tests. Phase 06 runs the entire suite from a clean, reproducible environment.

## Phase Documents

- [Phase 00](phase-00-ai-repository-foundation.md)
- [Phase 01](phase-01-foundation-data.md)
- [Phase 02](phase-02-analytics-engine.md)
- [Phase 03](phase-03-backend-api.md)
- [Phase 04](phase-04-frontend-product.md)
- [Phase 05](phase-05-ai-investigation.md)
- [Phase 06](phase-06-quality-delivery.md)

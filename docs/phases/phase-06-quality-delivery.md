# Phase 06 — Quality + Observability + Delivery + Demo

- Status: **Planned / Not Started**
- Authorization: requires explicit user authorization.

## Objective

Quality hardening, complete regression, and delivery verification: a reproducible, observable, reviewed deliverable that an evaluator can run from a clean clone and understand in a 5–10 minute demo. **This is not the phase where tests are first written**; it executes and completes the suite built by earlier phases.

## Prerequisites

- Phases 01–05 Complete.
- Phase Entry Gate: complete existing automated suite, Docker/runtime baseline, known acceptance flow.

## Scope

- **Reproducible demo (FR-DEL-002):** one documented mechanism that starts dependencies, creates/migrates the database, imports the provided data, starts backend and frontend, and preferably resets the demo dataset predictably. The exact entry point is OD-19 (it must also work on the evaluator's OS).
- Docker Compose for PostgreSQL, API, and frontend; optional Ollama profile.
- GitHub Actions CI: backend build/vet/tests (with testcontainers), frontend build/lint/typecheck/tests, Playwright.
- Prometheus-compatible `/metrics`.
- Fill gaps demonstrated by the regression run; no new product features.
- README runbook, demo script and rehearsal, known limitations.

## Non-Goals

New product modules; infrastructure listed in `docs/product/out-of-scope.md`.

## Expected Validation

- Full regression from a clean environment.
- Structured logs, health/readiness, and metrics verified.
- Docker Compose from a clean environment; migrations from an empty database; deterministic seed/import; demo reset path.
- CI green; OpenAPI verified.
- Accessibility basics and responsive critical screens verified.
- Security/secrets hygiene review.
- Final E2E critical journey.
- Performance-sensitive queries reviewed (`docs/performance/data-query-strategy.md`).
- Actual git diff review; documentation finalized; demo rehearsal executed.

## Evidence Required

Entry Gate result; clean-clone run-through using only README instructions; full suite commands and counts; CI run; repository scan (no secrets, no evaluator file, no dataset special-casing); all traceability rows Verified or documented as limitations.

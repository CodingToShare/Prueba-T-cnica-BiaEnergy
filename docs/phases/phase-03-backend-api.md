# Phase 03 — Backend API + Analysis Orchestration

- Status: **Planned / Not Started**
- Authorization: requires explicit user authorization.

## Objective

The versioned, documented product API over PostgreSQL, with persisted analysis runs executed in the background and observable progress.

## Prerequisites

- Phase 02 Complete.
- Phase Entry Gate: everything from Phase 01, analytics unit tests, analytics acceptance scenarios, existing integration tests.

## Scope

- Migrations for `analysis_runs` and `anomalies` (evidence in JSONB where appropriate); `computed_status` persisted or derived per Phase 02 decision.
- Run orchestration per ADR-007: 202 + run id, staged progress, transactional result write, startup recovery, concurrency policy (OD-14).
- Endpoints from `functional-requirements.md`; minimal login (OD-12).
- API quality items:
  - `/api/v1` versioning;
  - request validation;
  - consistent error responses without internal details;
  - cancellation and timeouts on I/O and runs;
  - OpenAPI 3 document kept in the repository and consistent with handlers;
  - pagination, filtering, and sorting where needed (meters, readings ranges, anomalies);
  - structured logging context with request id and `analysis_id` correlation;
  - health/readiness maintained.
- No generic middleware framework; only the middleware actually needed (request id, logging, recovery, CORS, auth check).

## Non-Goals

Frontend, LLM provider, CI, metrics endpoint.

## Expected Validation

- Handler/application unit tests where valuable (validation, error mapping, run state transitions).
- API + real PostgreSQL integration tests: success and error paths per endpoint, validation, persistence, filters/sorting/pagination, run lifecycle including failure and recovery, health/readiness.
- OpenAPI consistency check.
- Functional API-level acceptance flow: trigger analysis → poll to completion → anomalies in priority order (M-109 first) → M-109 detail with evidence and recommended action.
- Regression: all analytics and Phase 01 checks.

## Evidence Required

Entry Gate result; exact commands and counts; API acceptance flow output; Exit Gate checks; traceability rows FR-API-001, FR-ANL-*, FR-DASH-001, FR-MTR-*, FR-ANOM-001, FR-AUTH-001 (backend) updated.

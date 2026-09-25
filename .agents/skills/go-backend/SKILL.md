---
name: go-backend
description: Implement or modify the Go API under src/backend — handlers, services, run orchestration, configuration, and error handling — idiomatically and within ADR-002 boundaries.
---

# Go Backend

## Use When

Any change under `src/backend` except the pure analytics engine internals (see `analytics-engine`).

## Responsibilities

Serve the versioned API, orchestrate analysis runs (ADR-007, ADR-009), keep `docs/api/openapi.yaml` in sync with the router, and compose platform concerns, with packages by capability (ADR-002).

## Required Rules

- Handlers: decode, validate shape, call a service, encode. No business/analytics rules or SQL in handlers.
- `context.Context` first parameter on I/O; honor cancellation and timeouts.
- Errors: wrap with `%w`; map to HTTP centrally to the standard error body; never expose internals.
- Persistence through sqlc-generated code over pgx for product queries (from Phase 03); explicit parameterized pgx batches for bulk ingestion (TD-18); transactions where multiple writes must be atomic.
- Interfaces declared by consumers, only at meaningful boundaries (persistence when it aids testing, `ExplanationProvider`, `Clock`).
- `slog` structured logging with request/run ids; config from environment with validation at startup.
- Background runs: bounded context, persisted progress, `FAILED` on error, recovery of orphaned runs at startup, cancellation on shutdown.
- `gofmt`, `go vet` clean.

## Prohibited

Global mutable state (other than wired dependencies in `main`); panics for control flow; ORMs; generic repositories; reflection-heavy frameworks; goroutines without lifecycle ownership.

## Completion Checklist

- `go build ./...`, `go vet ./...`, relevant tests pass.
- API contract and OpenAPI updated.
- Error and cancellation paths considered and tested.

---
name: observability
description: Add or change structured logging, health and readiness endpoints, and Prometheus-compatible metrics for the API and analysis runs.
---

# Observability

## Use When

Adding request handling, background work, external calls (database, LLM), or operational endpoints.

## Responsibilities

Make behavior diagnosable with minimal, proportionate tooling (architecture §9).

## Required Rules

- `slog` JSON logs with request id, route, status, duration; analysis runs log stage transitions and outcomes with run id.
- `/healthz` reports process liveness only; `/readyz` checks database reachability with a short timeout.
- `/metrics` exposes HTTP request count/latency, analysis run duration/outcome, and explanation fallbacks; keep label cardinality low (route templates, not raw paths or IDs).
- Log levels meaningful: errors actionable, info for lifecycle, debug off by default.

## Prohibited

Logging secrets, credentials, or full payloads; high-cardinality labels; tracing stacks or large observability platforms without an ADR.

## Completion Checklist

- New flows produce useful logs and metrics.
- Health/readiness behavior verified, including the database-down case.

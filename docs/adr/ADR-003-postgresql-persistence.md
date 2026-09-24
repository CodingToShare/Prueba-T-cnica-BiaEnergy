# ADR-003: PostgreSQL As Primary Persistence

- Status: Accepted
- Date: 2026-09-24

## Context

The platform stores meters, hourly readings, events, analysis runs, anomalies, and structured evidence. It needs relational integrity, efficient range queries by meter and time, server-side filtering/sorting, and some flexibility for evidence payloads whose shape will stabilize during analytics work.

## Decision

- PostgreSQL is the single source of truth.
- Initial relational concepts: `meters`, `readings`, `events`, `analysis_runs`, `anomalies`. Exact schema is defined in Phase 01 (source data) and Phase 03 (analysis results).
- Timestamps use `timestamptz`. Amended by ADR-008: source observation times are timezone-naive `timestamp` values.
- Structured anomaly evidence may use JSONB where flexibility is valuable. Fields that are filtered, sorted, joined, or aggregated (meter, run, classification, severity, confidence, priority, status, timestamps) are proper columns.
- Indexes follow real query shapes, starting with `(meter_id, timestamp)` for readings; others are added when an actual query needs them (`docs/performance/data-query-strategy.md`).
- Integration tests run against real PostgreSQL via testcontainers-go.

## Consequences

- Strong integrity and a mature toolchain (pgx, sqlc, goose) with no additional infrastructure.
- JSONB evidence must be versioned or validated in code to avoid silent shape drift.
- Integration tests require Docker locally and in CI.

## Alternatives Considered

- **TimescaleDB**: valuable at much larger volumes; unjustified for 4,032 rows. Documented as an evolution path only.
- **SQLite**: simpler, but weaker for concurrent background runs and diverges from a production-like setup.
- **Document store**: loses relational integrity for data that is inherently relational.

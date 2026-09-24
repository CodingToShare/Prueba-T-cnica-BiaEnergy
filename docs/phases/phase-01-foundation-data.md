# Phase 01 — Runtime Foundation + PostgreSQL + Dataset Ingestion

- Status: **Planned / Not Started**
- Authorization: requires explicit user authorization.

## Objective

A runnable Go service with configuration, structured logging, health/readiness, a PostgreSQL schema for source data, and a verified, idempotent import of `readings.csv` and `events.csv`.

## Prerequisites

- Phase 00 Complete.
- `readings.csv` and `events.csv` placed unchanged in `data/input/` by a person.
- Phase Entry Gate: repository and governance validation (see roadmap), plus re-running the toolchain checks in `docs/quality/environment-readiness.md` (Go 1.27.x, Docker/Compose, `postgres:18.6-alpine` smoke).

## Scope

- Initialize the Go module in `src/backend`; select and pin chi, pgx, sqlc, goose, testify, testcontainers-go.
- `cmd/api`: HTTP server, graceful shutdown, `slog` JSON logs, environment config validated at startup, `/healthz`, `/readyz`.
- Docker Compose service for PostgreSQL only; `.env.example` with placeholders.
- goose migrations for `meters`, `readings`, `events`: keys, `NOT NULL`, uniqueness of `(meter_id, timestamp)`, numeric range checks where justified, `timestamptz`, `source_status` column (distinct from the future `computed_status`).
- sqlc configuration and first queries.
- `cmd/seed` ingestion:
  - source CSV preservation (read-only; checksum recorded before/after);
  - header and parsing validation; timestamp validation for both formats; numeric validation;
  - duplicate protection; gap detection and report;
  - idempotent import (re-run changes nothing);
  - timezone semantics applied per AS-02 in one place.
- Establish and document the first concrete build/test/seed commands and the first reproducible startup steps (OD-19).

## Non-Goals

Anomaly analysis, analysis tables, product API endpoints, frontend, CI, metrics endpoint, LLM.

## Expected Validation

- Go build; gofmt and go vet (static checks).
- Ingestion unit tests (parsing, validation, timestamp handling, duplicate detection).
- Database integration tests on real PostgreSQL: migrations from empty database, constraints enforced.
- Idempotent import verification.
- Dataset acceptance: exactly 12 meters, 4,032 readings, 14 days of hourly data (24 readings/meter/day), all events loaded; validation report (gaps, duplicates, ranges, `source_status` distribution).
- No source-file mutation (checksums unchanged).
- Health/readiness smoke, including readiness failing when the database is unavailable.

## Evidence Required

Entry Gate result; exact commands and counts; validation report; Exit Gate checks; traceability rows FR-DATA-001/002, FR-TECH-001 updated; resolutions of AS-02, AS-04, AS-05 recorded.

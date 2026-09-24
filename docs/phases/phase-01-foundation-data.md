# Phase 01 — Runtime Foundation + PostgreSQL + Dataset Ingestion

- Status: **Complete**
- Authorization: explicitly authorized by the user (pre-phase checkpoint commit + Phase 01 only).
- Date: 2026-09-24
- Foundation checkpoint: local commit `f42913c` "chore: establish Bia Energy engineering foundation" (not pushed; no remote).

## Objective

A runnable Go service with configuration, structured logging, health and readiness, a PostgreSQL schema for the source data, and a verified, idempotent import of `readings.csv` and `events.csv`. No analytics.

## Phase Entry Gate

| Check | Result |
| --- | --- |
| Phase 00 Complete; Phase 01 Planned | Yes (roadmap and phase documents) |
| Checkpoint | `f42913c` created from 58 reviewed files. Pre-commit checks: legacy scan (one false positive: "pending reboot"), secrets scan clean, no artifacts or `.env`, nothing ignored; tree clean afterwards |
| Toolchain (environment readiness) | Go 1.27.1; Docker Engine 29.8.0 / Desktop 4.91.0; Compose 5.5.1; `postgres:18.6-alpine` present |
| Ports | 5432 and 8080 free |
| Source files | Not in the repository; found as the supplied files in the user's Downloads folder, matching the contract (headers, 4,032 rows, 12 × 336, range, 4 events, LF, no BOM) |

## Decisions Made

| Decision | Record |
| --- | --- |
| Source timestamps stored timezone-naive (`timestamp without time zone`) with no zone invented; amends ADR-003's clause for source observations; AS-02 resolved | ADR-008, TD-12 |
| Measurements as `NUMERIC` (exact) in PostgreSQL and `float64` in Go; exactness proven for every source value | TD-17 |
| No physical-range constraints; only structural checks (not blank, finite, keys, FKs) | Data model |
| Natural keys: `meters.meter_id` PK; `readings (meter_id, reading_timestamp)` PK; events identity PK + natural unique key; no extra indexes (keys cover meter + time) | Data model |
| `event_type` is free text, not an enum | Data model |
| sqlc deferred to Phase 03; ingestion uses explicit parameterized pgx batch statements | TD-18 |
| goose used as a library through `cmd/migrate`, pinned in `go.mod` | TD-19 |
| Integration tests beside their packages with the `integration` build tag; `tests/integration/` placeholder removed | TD-20 |
| Module path `bia-energy.local/backend` (no remote exists; placeholder is easy to rename) | TD-21 |
| Backend container deferred to Phase 06 (the Go commands run on the host); Compose runs PostgreSQL only | Scope decision |
| Phase 01 developer commands are plain `docker compose` and `go -C src/backend run …`; single entry point stays open | OD-19 (updated) |
| AS-04 confirmed: dataset complete and regular | Evidence below |

## Implementation

| Area | Files |
| --- | --- |
| Source data | `data/input/readings.csv`, `data/input/events.csv` (byte-identical copies), `data/input/README.md` |
| Schema | `database/migrations/00001_create_source_data.sql` (up and down) |
| Runtime | `compose.yaml` (PostgreSQL 18.6, localhost-only port, healthcheck, named volume at the PG18 path), `.env.example` |
| Go module | `src/backend/go.mod`: chi v5.3.2, pgx v5.11.0, goose v3.28.0, testify v1.12.1, testcontainers-go v0.44.0 (test-only indirect `x/crypto` v0.56.0 and `moby/go-archive` v0.3.0 after the audit) |
| Commands | `cmd/api` (health/readiness, request logging, timeouts, graceful shutdown), `cmd/migrate` (up/down/status), `cmd/seed` (validated import + report) |
| Packages | `internal/config`, `internal/workspace`, `internal/ingestion` (parse, timestamp, report, files, store), `internal/platform/postgres` (+ `pgtest`), `internal/platform/health` |
| Tests | 6 unit test files, 7 integration test files (including the black-box `functional` package), the `pgtest` helper, synthetic fixtures in `internal/ingestion/testdata/valid` |
| Docs | ADR-008, `docs/architecture/data-model.md`, README, decision register, traceability, governance alignment (AGENTS, skills, ADR-002/003, architecture, testing strategy, performance) |

## Source File Integrity

| File | SHA-256 (original = copy) |
| --- | --- |
| `readings.csv` | `01c953c2daa6503f9696a46096f34c635d67b7394054ad3f628a0230393b162f` |
| `events.csv` | `750d42f11c2c409af7094153fd07071d3aa3b5a8d68bb418c9fcaf7cbb0c9932` |

`cmp` confirmed the copies are byte-identical. The acceptance test re-hashes both files after importing and asserts they are unchanged.

## Automated Tests (final run after the independent audit, `-count=1`)

| Level | Command (repository root) | Result |
| --- | --- | --- |
| Unit | `go -C src/backend test ./...` | PASS — 27 top-level tests and 25 subtests, 0 failed, 0 skipped; also PASS with `-shuffle=on` and with `-count=3`; no Docker needed |
| Integration + acceptance + functional | `go -C src/backend test -tags=integration ./...` | PASS — 41 top-level tests (27 unit + 14 integration) and 32 subtests, 0 failed, 0 skipped; also PASS with `-shuffle=on` and 8 consecutive parallel runs |
| Race detector | `go test -race` in the official `golang:1.27.1` Linux container (native Windows lacks cgo/gcc) | PASS — unit, unit shuffled, the integration suite, and the black-box test; no data races |

Integration tests (real PostgreSQL 18.6 via testcontainers):

- `TestMigrations_EmptyDatabase_UpDownUp`, `TestMigrations_SchemaMatchesSourceContract`
- `TestLoad_Fixtures_InsertsThenIsIdempotent`, `TestLoad_ChangedSourceValue_UpdatesOnlyThatReading`, `TestLoad_PersistsSourceValuesAndWallClockExactly`, `TestLoad_FailureMidway_WritesNothing`, `TestSchema_EnforcesIntegrityInTheDatabase` (7 cases: duplicate key 23505, foreign key 23503, check 23514; unusual values and new event types accepted)
- **Acceptance:** `TestAcceptance_SuppliedDataset_LoadsCompletelyAndIdempotently`. It covers:
  - 12/4,032/4; 336 per meter for M-101…M-112; min and max timestamps; 336 distinct hours; 0 gaps;
  - every one of the 4,032 readings equal to its source row (meter, timestamp, the four measurements decimal-equal, source status);
  - all 4 events equal to the source field by field;
  - second import 0 inserted / 0 updated, and no row rewritten (identical `ctid`/`xmin` snapshot);
  - source hashes unchanged.
- **Command-level, in-process** (they call each command's `run` function; the API test uses a real listening socket and real HTTP but not a separate process): `TestSeed_SuppliedDatasetTwice_KeepsExactCounts`, `TestSeed_InvalidInput_WritesNothing`, `TestMigrateCommand_UpStatusDownUp`, `TestAPI_HealthReadinessAndGracefulShutdown` (200/200 → DB stopped → `/readyz` 503, `/healthz` 200 → context cancel → shutdown returns nil), `TestAPI_DatabaseUnreachableAtStartup_FailsFast` (no password in the error).
- **Black-box functional:** `TestBlackBox_MigrateSeedServe_OverRealProcessesAndHTTP` (`src/backend/functional`).
  - It builds the real `api`, `migrate` and `seed` executables with `go build` and runs them as separate OS processes against a disposable PostgreSQL container: migrate up, seed twice (12/4,032/4 each time), then start the API on a dynamic port.
  - Over real HTTP it checks `/healthz` 200, `/readyz` 200, product route 404, `/readyz` 503 while PostgreSQL is paused (`docker pause`), `/healthz` still 200, and `/readyz` back to 200 after unpause.
  - Shutdown: on Linux/macOS the API gets SIGTERM and must exit 0 with "shutdown complete" (executed in the Linux container, PASS). On Windows it is killed, because a test cannot deliver Ctrl+C to a child process.
  - It uses the fake password `SUPER_SECRET_AUDIT_VALUE` and asserts it appears nowhere in any process output.
  - It also exercises path resolution from a directory two levels below the repository root.

Test containers (including the Ryuk reaper) were removed after the runs.

## Clean-Database Runtime Validation

Run with compiled binaries against the Compose database, after checking that no other Docker volumes or containers existed:

1. `docker compose down -v` → `docker compose up -d --wait` → healthy in 6 s (`postgres:18.6-alpine`, `127.0.0.1:5432`).
2. `migrate status` → `pending 00001`; `migrate up` → `migrations applied count=1 versions=[1]`; `status` → `applied`.
3. First `seed`: `source files validated readings=4032 events=4 meters=12 first_reading="2026-09-01 00:00:00" last_reading="2026-09-14 23:00:00" hourly_gaps=0 irregular_steps=0 source_statuses="OK=4032" event_types="DATA_QUALITY=1 OPERATIONAL_CHANGE=1 SCHEDULED_OUTAGE=1 UNKNOWN=1"`, then `import complete meters=inserted 12, readings=inserted 4032, events=inserted 4, duration_ms=874`.
4. Second `seed`: `inserted=0 updated=0 unchanged=12 / 4032 / 4`, duration 522 ms.
5. SQL: meters 12, readings 4032, events 4; readings per meter min = max = 336 across 12 meters; range `2026-09-01 00:00:00` to `2026-09-14 23:00:00`; meter IDs `M-101…M-112`.
6. API binary:
   - `/healthz` → 200 `{"status":"ok"}`; `/readyz` → 200 `{"status":"ready","checks":{"postgres":"ok"}}`; `/api/v1/meters` → 404.
   - `docker compose stop postgres` → `/readyz` 503 `not_ready`, `/healthz` 200; after `start` → `/readyz` 200.
   - Logs are JSON; the database URL is logged with its password masked; data persisted (4,032 readings).
7. README commands re-verified in PowerShell and Bash (`go -C src/backend run …`, `test ./...`).

Import durations are informational only, not a benchmark.

## Phase Exit Gate

| Check | Result |
| --- | --- |
| Formatting | `gofmt -l .` clean (one alignment finding in an integration test fixed; package re-tested) |
| Static analysis | `go vet ./...` and `go vet -tags=integration ./...` OK; `go mod verify` all modules verified |
| Build | `go build ./...` OK |
| Unit / integration / acceptance / functional | PASS (see above) |
| Database validation | Migrations from an empty database, down and up again, constraints verified |
| Runtime smoke | PASS (clean-database run) |
| Secrets and config | No tokens or keys. Credential-shaped strings are only documented local placeholders (`bia_local_dev` in README, `.env.example`, `compose.yaml`) and fake test values; `.env` ignored; the password is masked in logs and errors |
| Scope review | No analytics code, no product API (only `/healthz`, `/readyz`), `src/frontend` untouched, no meter IDs in production code, no absolute paths, no TODOs, no evaluator-file reference |
| Diff review | `git status` / `git diff --stat` reviewed; one broken table separator in the testing strategy fixed |
| Documentation and traceability | Updated in this change |
| Linting beyond vet | N/A: no linter is configured or approved (golangci-lint not adopted) |
| Playwright / UI design validation | N/A: no frontend exists in Phase 01 |

## Known Limitations And Deferred Work

- Analytics and anomaly detection are intentionally Phase 02; nothing in Phase 01 inspects the acceptance meters beyond checking their identifiers exist.
- Graceful shutdown of the real process (SIGTERM → exit 0) is proven by the black-box test on Linux. On native Windows it is proven in-process (context cancellation), because a test cannot deliver Ctrl+C to a child process.
- The chi request ID includes the host name (library default); consider a plain random ID when the product API arrives (Phase 03).
- Backend container, single entry-point script, demo reset and CI belong to Phase 06; sqlc to Phase 03; `computed_status` to Phases 02–03.
- Trailing zeros of source decimals are not kept (`222.0` → `222`); the values are numerically identical.

## Independent Completion Audit (2026-09-24)

Phase 01 was re-verified from evidence, not from the earlier report.

| Finding | Severity | Resolution |
| --- | --- | --- |
| Integration suite was flaky on Windows: 3 of 6 parallel runs failed in testcontainers Docker detection. Cause: testcontainers calls `os.Stat` on the `docker_engine` named pipe, which fails while the pipe is busy with concurrent test binaries. Earlier green runs were not reproducible evidence | Important | `pgtest` sets `DOCKER_HOST` to the Docker SDK default on Windows when unset (respects explicit settings). 8 of 8 subsequent parallel runs (plain and shuffled) passed |
| "Functional" tests called `run()` in-process; no test ran the compiled executables | Important | Added the black-box test (real processes, real HTTP, real PostgreSQL, pause/unpause, SIGTERM on Linux, password-leak check) |
| Event fields were never compared with the source; "unchanged rows are not rewritten" was proven only by returned counts | Important | Acceptance test now compares all event fields and asserts an identical `ctid`/`xmin` snapshot after re-import |
| Test-only dependencies had 3 known vulnerabilities reachable from test code (GO-2026-6355, GO-2026-6354 in `x/crypto` v0.55.0; GO-2026-6253 in `moby/go-archive` v0.2.0); production binaries unaffected | Minor | Upgraded to the fixed versions; `govulncheck`: no vulnerabilities in production code, none reachable from tests |
| `HTTP_ADDR` was not validated; a bad value failed only after connecting to PostgreSQL | Minor | Validated at startup (host:port or :port, 0–65535) with unit tests |
| Reading write errors did not name the failing row | Minor | Errors now name meter and timestamp; asserted by the atomicity test |
| Unused `Migrator.DownTo` (dead code) | Minor | Removed |
| Untested branches: missing timestamp value, more than 25 errors, missing migrations directory | Minor | Unit tests added |
| Phase record miscounted test files and overstated the in-process tests as functional | Minor | Corrected above |

Verified without change:

- **Module:** no `tidy` drift; `go mod verify`; each command builds; production binaries link 10–11 modules (no test dependencies).
- **CSV bytes:** proven stable across checkout even with `core.autocrlf=true` (active on this machine). Without `data/input/*.csv -text`, checkout rewrote 4,033 lines to CRLF and changed both hashes; with it, the bytes are identical.
- **Full hashes unchanged:** readings `01c953c2daa6503f9696a46096f34c635d67b7394054ad3f628a0230393b162f`, events `750d42f11c2c409af7094153fd07071d3aa3b5a8d68bb418c9fcaf7cbb0c9932`.
- **Schema by direct SQL:**
  - four key indexes only, and the meter + range query uses `readings_pkey`;
  - FKs are NO ACTION (no cascade);
  - check constraints are structural only;
  - 0 duplicates, 0 orphan readings, 0 orphan events.
- **README lifecycle:** executed in PowerShell from a reset volume (down -v, up --wait, migrate, seed, API health/readiness, re-import, commands from `src/backend`, down). No password in logs.
- **Coverage (visibility only):** 83.8% combined in-process; health 100%, config 96.8%, ingestion 83.7% (unit).

## Next Phase

None authorized. Phase 02 requires explicit user authorization and starts with its Entry Gate (backend build, vet, unit and integration suites above).

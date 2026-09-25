# Phase 03 — Backend API + Analysis Orchestration

- Status: **Complete (independent audit; non-blocking notes)**
- Authorization: explicitly authorized by the user, together with the Phase 02 checkpoint commit, including a continuation that assigns the minimal **backend** login to this phase. Phase 04 (frontend, login UI) is not authorized and was not started.
- Date: 2026-09-24
- Phase 02 checkpoint: local commit `6c92a6b` "feat: implement deterministic anomaly analysis engine" (not pushed; no remote).

## Objective

Turn the Phase 02 engine into an application capability: PostgreSQL source data → versioned REST API → persisted analysis runs executed in the background → persisted findings and evidence → queryable meters, anomalies and dashboard, with status suitable for frontend polling and a minimal demo login.

## Phase 02 Checkpoint And History Reconciliation

- **Phase 01 hashes:** `a1f1542` (reported originally) and `a81f38e` (current) both exist. The reflog shows `rewrite: remove AI co-author trailer`. It was requested by the owner to remove an assistant attribution trailer from the two local commits.
  - Root commit: same tree as before.
  - `a81f38e` vs `a1f1542`: only the two Phase 00 checkpoint hash references in `docs/phases/phase-01-foundation-data.md` differ.
  - No Phase 02 file is in the Phase 01 commit. The effective Phase 01 checkpoint is `a81f38e`.
- **Change set checked before the Phase 02 commit:**
  - 20 new files (Phase 02 production code and tests, including the independent audit's `audit_test.go`);
  - 15 modified documents (Phase 02 documentation, plus the owner-requested commit-attribution rule in `AGENTS.md`, `CLAUDE.md` and the phase-governance skill).
  - No binaries, coverage, `.env`, secrets, database data, expected-results file, frontend, API or migration. The ignored `tmp/` audit artifacts were not committed.
- **Pre-commit regression:**
  - `mod verify`, `vet`, unit and `diff --check` all passed;
  - integration: 127 top-level tests, 0 failed, 0 skipped;
  - the supplied-data acceptance confirmed M-109 REAL/HIGH, M-112 DATA_QUALITY/HIGH, M-104 EXPLAINABLE/MEDIUM, M-106 FALSE_POSITIVE/LOW in that priority order.
- **Commit:** `6c92a6b`, author and committer the repository owner, no attribution trailer. The working tree was clean afterwards (only the ignored local audit artifacts remain).

## Phase Entry Gate

| Check | Result |
| --- | --- |
| Phase 02 Complete; Phase 03 authorized | Yes |
| Governing documents read | AGENTS, project context, skills (phase-governance, architecture, go-backend, postgresql, testing, code-review, documentation, observability), this document, product requirements, business rules, traceability, architecture, data model, `docs/ai/*`, ADR-002/003/006/007/008, testing strategy, DoD |
| Baseline on `6c92a6b` | Tree clean; `go build ./...` and `go vet -tags=integration ./...` OK; unit PASS; integration PASS: 127 top-level tests and 83 subtests, 0 failed, 0 skipped |

## Decisions

| Decision | Record |
| --- | --- |
| PostgreSQL-backed queue with atomic claim (`FOR UPDATE SKIP LOCKED`); one active run via a partial unique index; truthful coarse stages; startup marks only interrupted `RUNNING` runs `FAILED`; 2-minute run timeout; worker stops after HTTP on shutdown | ADR-009 (amends ADR-007), TD-25, OD-14 |
| sqlc 1.31.1 from the pinned official image (digest `sha256:70f53171d27b2424e9358869975455a6e955a5aa8e58a998a270a6e34e525537`); SQL in `database/queries`; generated package `dbgen` | TD-22 (TD-18 realized) |
| Engine version and configuration snapshot stored per run (JSON tags on `analysis.Config`; `analysis.EngineVersion = "1.0.0"`; `Engine.Config()` accessor). No change to the algorithm | TD-23 |
| API JSON conventions: snake_case, source times without offset, system instants RFC 3339 UTC, percent vs fraction, explicit nulls | TD-24 |
| Minimal backend login | OD-12 (backend) |
| Aggregate confidence is the mean over the latest completed run (`null` without findings); high priority = severity HIGH | OD-07 |
| The engine's per-meter status is persisted per run and exposed | OD-10 |
| Meter and dashboard consumption windows | OD-06 |
| POST while a run is active returns that run (202, `created: false`) instead of 409 | ADR-009 |
| Anomaly lifecycle `status` fixed to `OPEN` (check constraint), with no workflow | Data model |
| Prometheus `/metrics` stays in Phase 06 (phase non-goal); no CORS middleware (Phase 04 prefers a same-origin proxy) | Scope |
| New dependencies: none. `go.yaml.in/yaml/v3` (already in the module graph through testify, same version) became a direct test dependency for the OpenAPI checks | `go.mod` |

## Implementation

| Area | Files |
| --- | --- |
| Migration | `database/migrations/00002_create_analysis_results.sql`: `analysis_runs`, `anomalies`, `analysis_meter_results`, CHECK constraints, two partial indexes |
| Queries | `database/queries/{analysis_runs,meters,anomalies,dashboard}.sql`; `sqlc.yaml`; generated `internal/platform/postgres/dbgen` (6 files) |
| Orchestration | `internal/analysisrun`: `model.go` (statuses, stages, progress, failure codes, `Analyzer` boundary), `service.go` (request, get, recovery, worker, claim, execution, atomic persistence), `evidence.go` (typed evidence document, schema version 1), `current.go` (latest completed / active run) |
| Read services | `internal/meter`, `internal/anomaly`, `internal/dashboard`; `internal/platform/postgres/snapshot.go` (read-only REPEATABLE READ snapshot per request) |
| Authentication | `internal/auth` (credential check, HMAC-SHA256 session token, cookies); `internal/config` `LoadAPI` (auth settings, API only) |
| HTTP | `internal/httpapi`: router, middleware (host-independent request ID, structured request log, JSON panic recovery, 15 s request timeout, session check), error contract, validation, DTOs; `internal/platform/jsontime` (source vs system time JSON) |
| Wiring | `cmd/api/main.go` (auth, pool, engine, recovery, worker lifecycle, shutdown order); the old `cmd/api/router.go` moved into `internal/httpapi` |
| Contract | `docs/api/openapi.yaml` (OpenAPI 3.1) |

The Phase 02 engine is unchanged in semantics and still imports only the standard library. The analytics package never touches PostgreSQL: orchestration → engine, and orchestration → sqlc → PostgreSQL.

## Authentication

| Property | Value |
| --- | --- |
| Routes | `POST /api/v1/auth/login` (public), `POST /api/v1/auth/logout` (public, idempotent, 204), `GET /api/v1/auth/session` (protected) |
| Credential | `DEMO_AUTH_USERNAME` / `DEMO_AUTH_PASSWORD` (password ≥ 8 characters), compared in constant time over SHA-256 digests, both fields always evaluated |
| Session | `base64url(JSON{sub, iat, exp, nonce}) "." base64url(HMAC-SHA256)`, key `SESSION_SIGNING_KEY` (≥ 32 bytes). Signed, not encrypted; holds no secret. 8-hour lifetime |
| Cookie | `bia_session`; HttpOnly; SameSite=Lax; Path=/; Max-Age 8 h; Secure unless `SESSION_COOKIE_SECURE=false` (default `true`) |
| Verification | Signature first (`hmac.Equal`), then subject = configured user, expiry, lifetime ≤ 8 h, not issued in the future, no unknown claims |
| Behavior | 401 standard error for any protected `/api/v1` route without a valid cookie (never a redirect); login failure is 401 `invalid_credentials` with a generic message; `/healthz` and `/readyz` stay public |
| Startup | The API refuses to start when the auth settings are missing or weak, without echoing the values. `migrate` and `seed` do not need them |
| Limits (documented) | No user table, roles, registration, rate limiting or server-side session store: logout clears the cookie but cannot revoke a copied token before it expires. CSRF: SameSite=Lax plus a JSON API with no cross-site frontend. Revisit cross-site cookies only in the delivery phase |

## Analysis Orchestration

- **POST policy:** `POST /api/v1/ai/analyze` inserts a QUEUED run (`ON CONFLICT DO NOTHING` against `analysis_runs_single_active`). If a run is already active, that run is returned with `created: false`. Both cases answer 202 with a `Location` header and never wait for the analysis.
- **Worker:** one goroutine owned by `cmd/api`. It is woken by the request and also polls every 10 s. It claims the oldest QUEUED run atomically, then:
  - loads readings and events from PostgreSQL (`::float8`) and records the source counts;
  - runs `Engine.Analyze` through `Analyzer`;
  - writes meter statuses, findings and `COMPLETED` in one transaction.
- **Stages and progress:** QUEUED 0 → LOADING_DATA 10 → ANALYZING 35 → PERSISTING_RESULTS 85 → COMPLETED 100, written at each real transition. A failed run keeps the progress of the stage that failed.
- **Failures:** codes `load_failed`, `analysis_failed`, `persistence_failed`, `timeout` and `interrupted`, each with a fixed message. They are recorded even after cancellation, through a separate 5 s operation.
- **Timeout:** 2 minutes per run.
- **Shutdown:** stop HTTP (10 s graceful), cancel the worker, wait for it, then log "shutdown complete".
- **Startup:** RUNNING runs become FAILED `interrupted`; QUEUED runs are kept and processed.
- **Logs:** each run logs these structured events:
  - "analysis queued / started / source data loaded / engine completed / findings persisted / analysis completed", with `analysis_id`, counts and `duration_ms`;
  - "analysis failed", with `error_code`.
  - No readings or evidence are logged.

## API

| Method | Path | Auth | Purpose | Success |
| --- | --- | --- | --- | --- |
| GET | `/healthz` | Public | Liveness | 200 |
| GET | `/readyz` | Public | PostgreSQL readiness | 200 / 503 |
| POST | `/api/v1/auth/login` | Public | Start demo session | 200 + cookie |
| POST | `/api/v1/auth/logout` | Public | Clear session cookie | 204 |
| GET | `/api/v1/auth/session` | Session | Current session | 200 |
| GET | `/api/v1/meters` | Session | Search, status filter, sort, pagination | 200 |
| GET | `/api/v1/meters/{meterId}` | Session | Meter detail and current finding | 200 |
| GET | `/api/v1/meters/{meterId}/readings` | Session | History `[from, to)`, limit ≤ 1000 | 200 |
| GET | `/api/v1/anomalies` | Session | Current findings by priority; filters; pagination | 200 |
| GET | `/api/v1/anomalies/{id}` | Session | Finding with structured evidence | 200 |
| POST | `/api/v1/ai/analyze` | Session | Queue analysis (or return the active run) | 202 |
| GET | `/api/v1/ai/analysis/{id}` | Session | Run status, stage, progress, counts, error | 200 |
| GET | `/api/v1/dashboard/summary` | Session | Fleet summary | 200 |

**Error contract:** `{"error":{"code","message","request_id"}}` with the same `X-Request-ID` header. The ID is 128 random bits in hex, contains no host name, and incoming values are not trusted.

| Status | Codes |
| --- | --- |
| 400 | `invalid_query` (including a malformed query string and repeated parameters), `invalid_meter_id`, `invalid_anomaly_id`, `invalid_analysis_id`, `invalid_timestamp`, `invalid_range`, `invalid_request` |
| 401 | `unauthorized`, `invalid_credentials` |
| 404 | `*_not_found` and `not_found` |
| 405 | `method_not_allowed` |
| 503 | `request_timeout` |
| 500 | `internal_error` |

There are no HTML error pages.

## Implementation Validation (before the independent audit)

The independent audit below supersedes these earlier counts and race results.

| Level | Command (repository root) | Result |
| --- | --- | --- |
| Unit (no Docker) | `go -C src/backend test ./...` | PASS: 148 top-level tests and 76 subtests, 0 failed, 0 skipped; also PASS with `-shuffle=on`. New in Phase 03: auth 8, jsontime 4, config +3, analysisrun 5, httpapi 15 (incl. 4 OpenAPI checks) |
| Integration + black-box | `go -C src/backend test -tags=integration ./...` | PASS: 184 top-level tests and 85 subtests, 0 failed, 0 skipped, PostgreSQL 18.6 via testcontainers. New in Phase 03: analysisrun 12, httpapi API 8, the golden-path black-box test, and an auth-config test in `cmd/api` |
| Race detector | `go test -race ./...` and `go test -race -tags=integration ./...` in `golang:1.27.1` plus the official Docker CLI (the Phase 02 audit procedure), `TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal` | PASS for every package, no data race. This includes the golden path on Linux with SIGTERM → exit 0, "analysis worker stopped" and "shutdown complete". Run before the last two test additions; no production code changed after it |
| Static | `go mod tidy` (no change), `go mod verify`, `gofmt -l`, `go vet ./...`, `go vet -tags=integration ./...`, `go build ./...`, `git diff --check` | Clean |
| sqlc | `docker run --rm -v "<repo>:/src" -w /src sqlc/sqlc:1.31.1 generate` twice; `… diff`; `… vet` | Second generation byte-identical (6 files, SHA-256 compared); `diff` and `vet` exit 0 |
| Coverage (visibility) | `-coverpkg` over the Phase 03 packages, unit + integration | 88.0% of statements. analysisrun 88%, auth 98%, httpapi 94%, meter 89%, dashboard 97%, jsontime 96%. The two risky low branches found (run no longer RUNNING; 500/503 mapping) got dedicated tests. `cmd/api` `main()` is covered by the black-box processes, not measurable in-process |

**Key evidence by requirement:**

- **Migration:** `TestMigrations_EmptyDatabase_UpDownUp` covers 00001 + 00002 up, down twice (analysis tables go first, then source), and up again.
- **Evidence round trip:** `TestRun_EvidenceRoundTripsThroughJSONB`. For every supplied-data finding, the JSONB decodes to a document identical to `EvidenceOf(engine finding)`. It checks:
  - M-109: consumption +110%, current UP corroborates, PF DOWN corroborates, 58 h sustained, no recovery, UNKNOWN as CONTEXT, confidence breakdown, INVESTIGATE action;
  - M-112: consumption never triggered, voltage and current corroborate, DATA_QUALITY event CORROBORATES.
- **Same results through PostgreSQL:** `TestRun_SuppliedDatasetFromPostgreSQL_CompletesWithPrioritizedFindings`.
  - Analyzing the data read from PostgreSQL gives exactly the CSV-path types, severities and confidences, bit-for-bit, in the order M-109, M-112, M-104, M-106.
  - Run: 12 meters, 4,032 readings, 4 events, 4 findings, 2 high priority. The configuration snapshot equals `DefaultConfig()`.
- **Failures:**
  - analyzer failure: FAILED `analysis_failed`, no rows, the previous completed run stays current;
  - persistence failure: a foreign-key violation after 12 statuses and 4 findings were inserted rolls back every row, the run fails `persistence_failed`, and nothing becomes current;
  - timeout;
  - interruption by shutdown;
  - a run failed by another actor mid-flight writes no results.
- **Concurrency:**
  - 12 concurrent `Request` calls share one run (1 created);
  - 8 concurrent HTTP POSTs share one run;
  - 20 rounds of two competing claims: exactly one wins every time.
- **Current state:** run A completed → B queued → B running → B failed: A stays current throughout. Then C completes and becomes current; A's rows are kept (`TestLatestCompleted_RunningOrFailedRunsNeverReplaceIt`, `TestAPI_AnalysisLifecycleAndCurrentState`).
- **API:** one integration test per endpoint covering success, filters, sorting, pagination, 404 and 400. JSON-contract checks:
  - source times without `Z`;
  - system times RFC 3339 `Z`;
  - numeric confidence;
  - evidence as an object;
  - explicit nulls before analysis and for meters without a finding;
  - responses decoded with `DisallowUnknownFields`.
- **OpenAPI:**
  - the document parses; every `$ref` resolves;
  - router routes ⇔ spec operations (13, both ways);
  - public vs session security per operation, and 401 documented on every protected one;
  - 18 schemas list exactly the JSON fields of their DTOs.
  - A mutation check (removing a route, renaming a field) made the tests fail as intended.

## Black-Box Golden Path (`TestBlackBox_AuthenticatedAnalysisGoldenPath`)

The test builds `migrate`, `seed` and `api` with `go build`, runs each as a separate OS process against a disposable PostgreSQL 18.6 container, and uses real HTTP with a cookie jar. It never calls handlers or internals. Steps:

1. Migrate up, seed, start the API.
2. `/healthz` and `/readyz` answer 200; the dashboard answers 401.
3. A wrong password answers 401 `invalid_credentials`; the correct login sets the cookie.
4. Dashboard before analysis: 12 meters, total consumption equal to the SQL sum, 0 anomalies, 0 high priority, `aggregate_confidence` null, `latest_analysis` null.
5. `POST /ai/analyze` answers 202 with `Location` and `created: true`.
6. Bounded polling (60 s): the run is COMPLETED with progress 100, 4 findings, 2 high priority, 4,032 readings, and `completed_at` ending in `Z`.
7. `/anomalies` returns M-109, M-112, M-104, M-106. The M-109 detail shows REAL/HIGH, the INVESTIGATE action, `started_at` `2026-09-12T14:00:00`, consumption +110%, current and PF corroborating, and the UNKNOWN event as CONTEXT.
8. `/meters`: M-109 CRITICAL, M-112 ALERT, M-104 ALERT, M-106 OK. M-109 detail and 336 readings: source status `OK` while the computed status is CRITICAL.
9. Dashboard after: 4 anomalies, 2 high priority, the run's aggregate confidence, and the latest analysis COMPLETED.
10. Logout answers 204; `/anomalies` answers 401.
11. On Linux, SIGTERM gives exit 0 with "analysis worker stopped" and "shutdown complete" (Windows kills the process; graceful shutdown is proven in-process).
12. The logs contain "analysis completed" with the `analysis_id`. No DB password, demo password, signing key or cookie value appears in any output.

The duration is about 9 s on Windows, including builds.

## Manual Acceptance (README walkthrough, Compose PostgreSQL)

- **Setup:** `docker compose up -d --wait`. `migrate up` applied version 2. `seed` reported everything unchanged (12/4,032/4). The compiled API ran with the `.env.example` placeholders.
- **curl sequence:** health, then 401, then login, then:
  - dashboard: 12 meters, 155,250.85 kWh, null analytics;
  - analyze: 202, `Location: /api/v1/ai/analysis/1`;
  - the run went QUEUED → COMPLETED in 41 ms (started_at to completed_at): 12/4,032/4 source counts, 4 findings, 2 high priority, aggregate confidence 0.8338;
  - anomalies: M-109 REAL/HIGH 0.933, M-112 DATA_QUALITY/HIGH 0.917, M-104 EXPLAINABLE/MEDIUM 0.728, M-106 FALSE_POSITIVE/LOW 0.756;
  - M-109 CRITICAL;
  - logout 204, then 401.
- **Logs:** the lifecycle messages above, 0 occurrences of any configured secret.
- **Teardown:** `docker compose stop` (volume kept).

## Persisted Results On The Supplied Data

| Priority | Meter | Type | Severity | Confidence |
| --- | --- | --- | --- | --- |
| 1 | M-109 | REAL_ANOMALY | HIGH | 0.9333 |
| 2 | M-112 | DATA_QUALITY | HIGH | 0.9173 |
| 3 | M-104 | EXPLAINABLE_ANOMALY | MEDIUM | 0.7284 |
| 4 | M-106 | FALSE_POSITIVE | LOW | 0.7564 |

These equal the Phase 02 CSV-path results exactly. Dashboard: 12 meters, 155,250.85 kWh, 4 anomalies, 2 high priority, aggregate confidence 0.8338 (mean of the four).

## Query And Index Review

- **`EXPLAIN (ANALYZE)` on the seeded data with one completed run:**
  - reading history uses an Index Scan on `readings_pkey` (meter + half-open range), 3 buffers;
  - the meter list is one query: one aggregate pass over readings, joins to the current run's statuses and top findings, 42 buffers, about 1.3 ms, with no per-meter queries;
  - the current-anomalies list is one join, 2 buffers.
  - The latest-completed and active-run lookups use sequential scans of a 1-row table; the partial indexes matter as history grows, and the unique index is the concurrency guarantee.
- **No N+1:**
  - the meter list uses one list query plus one count query plus the latest-run lookup, inside one snapshot transaction;
  - the anomaly list reads stored columns without evidence;
  - the detail reads one row whose evidence (including events) is already stored.
- **Injection safety:**
  - every value is parameterized;
  - sort keys and directions reach SQL only as whitelisted values chosen in Go (`CASE` expressions);
  - search uses `strpos(lower(…))`, so `%` and `_` are literal (tested);
  - malformed query strings are rejected (a real defect found and fixed: `r.URL.Query()` silently dropped `;`-separated pairs).
- **Determinism:** anomalies are ordered by priority then id; meters by the chosen key then `meter_id`; pagination totals come from the same snapshot.
- **Indexes added (non-key):**
  - `analysis_runs_single_active` (partial unique, concurrency policy);
  - `analysis_runs_latest_completed` (partial, current-state lookup).
  - The `(analysis_run_id, priority)` unique constraint serves ordered findings per run. No other index was added.

## Code Review (actual diff)

- **Scope:** no frontend, Phase 05 AI code, LLM or broker; no Redis, Kafka, WebSocket/SSE, GraphQL, JWT/OAuth or generic job framework.
- **Engine:**
  - analytics are not duplicated: the API never recomputes, and the meter status is the engine's;
  - the engine is unchanged apart from JSON tags, the version constant and the configuration accessor;
  - no challenge meter ID appears in production code.
- **Data correctness:**
  - no fake progress or sleeps in production code;
  - source and computed status are kept separate (tested end to end);
  - source vs system time formats are tested at unit, API and black-box level.
- **Hygiene:**
  - generated files are sqlc-only and regenerate byte-identically;
  - no debug output; no machine-specific paths in tracked files.
- **Secrets:** no secret in the diff. `.env.example` and the README contain only local placeholders, `cookies.txt` is git-ignored, and passwords, keys and cookies never appear in logs (asserted).
- **Fixed during review:**
  - malformed query strings;
  - "shutdown complete" was logged before the worker stopped;
  - `min`/`max` over zero rows would have failed to scan (dedicated `GetReadingPeriod` query);
  - aggregate confidence is stored per run instead of a query sqlc typed as non-null.

## Phase Exit Gate

| Check | Result |
| --- | --- |
| Build, format, lint/vet, module | PASS |
| Unit, integration, black-box functional | PASS (counts above) |
| Race detector | PASS (Linux container) |
| Database validation | Migration 00002 up/down/up from an empty database; constraints and rollback tested on real PostgreSQL |
| Security and secrets | PASS (see review) |
| OpenAPI consistency | PASS |
| Manual acceptance | PASS (README walkthrough) |
| Playwright / design validation | N/A: no frontend in Phase 03 |
| Documentation and traceability | Updated: README, `.env.example`, `docs/api/openapi.yaml`, ADR-009 (+ ADR-007 and ADR-002 notes), architecture, data model, decision register (TD-22–25, OD-06/07/10/12/14), traceability, testing strategy, roadmap, project context, AGENTS routing, go-backend skill |
| Regression of Phases 01–02 | PASS. The Phase 02 engine suite is unchanged (86 tests) and the supplied-data results are identical |

## Known Limitations And Deferred Work

- **Single API process:**
  - startup recovery fails any RUNNING run, which is wrong if a second process is running it (ADR-009);
  - several processes would need a lease or heartbeat.
- **Demo authentication:**
  - one credential; no server-side revocation before expiry; no login rate limiting;
  - the session cookie is signed, not encrypted (it holds no secret).
- **Meter list performance:** the meter list aggregates readings on every request, which is fine for 4,032 rows. A stored per-meter total or materialized view waits for measured need (data query strategy).
- **Interrupted runs:** they are not retried automatically.
- **Phase 04:** login UI, dashboard UI, meter screens, charts, anomaly and investigation screens, responsive UX, frontend polling of `GET /api/v1/ai/analysis/{id}`, and CORS or a same-origin proxy.
- **Phase 05:** explanation presentation, the deterministic explanation provider, optional Ollama, and the deeper AI investigation experience. The evidence contract they consume is `Evidence` schema_version 1.
- **Phase 06:** Prometheus `/metrics`, CI, a single-command demo and reset (OD-19), and a backend container.

## Next Phase

None authorized. Phase 04 requires explicit user authorization and starts with its Entry Gate (roadmap "Phase 04" baseline commands).

## Independent Backend Audit

The independent audit was explicitly authorized for Phase 03 review, regression tests and targeted remediation only. HEAD remains `6c92a6b`; nothing was staged, committed or pushed. Phase 04 and Phase 05 were not started. Entry validation on the uncommitted implementation passed build, integration-tagged vet, and 184 top-level tests + 85 subtests with no failures or skips. The earlier implementation evidence above is historical; this section records the audited tree.

### Findings And Remediation

No critical defect was found. Reproduced defects were repaired with focused regressions before full validation:

| Finding | Resolution and evidence |
| --- | --- |
| A queued request surviving a deployment could execute the new engine while retaining the previous version/configuration | The atomic sqlc claim now stores the executing service's version and configuration. `TestRun_AuditQueuedRestartUsesActualEngineProvenance` failed before the fix and passes with a different engine configuration/version |
| Login accepted a second JSON document, trailing garbage and oversized whitespace after valid credentials; media type was unchecked | Require application/json and EOF after exactly one object, through the 4096-byte reader. Seven whole-body/media-type cases cover accepted JSON/charset and rejected input; unknown-field tests remain |
| Signed token lifetime arithmetic could overflow; signed payloads accepted trailing documents | Compare the positive lifetime in unsigned seconds and require EOF. Regression uses correctly signed crafted tokens: this was a claim-validation defect, not a signature bypass. Tests include exact maximum lifetime, overflow, extra JSON and the 60-second future-issue tolerance boundary |
| Panic values were logged verbatim and could contain credentials | Log panic type and server-side stack, excluding the value. The injected-password regression verifies the value is absent from logs and the standard 500 retains its request ID |
| JSON encoding failure lost the request ID | Build the fallback error using the response's request ID. A non-finite-number regression checks a complete JSON error with matching ID |
| Source-time parsing accepted fractional seconds and noncanonical hours despite the OpenAPI format | Require the parsed time to round-trip exactly to the source layout; offset, fraction, comma fraction and short-hour input are rejected |
| Search length counted UTF-8 bytes while OpenAPI specified characters | Count Unicode code points; real API tests accept 64 accented characters and reject 65 |

Readings and events previously used separate snapshots. The existing `postgres.ReadSnapshot` helper now loads both in one read-only REPEATABLE READ transaction, which closes before computation. A controlled intervening event commit proves snapshot isolation. This closes the source-consistency limitation without introducing new transaction infrastructure.

### Boundary, Data And Failure Review

- Dependency flow: `cmd/api` composes authentication, thin `httpapi` handlers, capability services, one shared PostgreSQL pool and one worker. Product SQL comes from `database/queries` via generated `dbgen`; ingestion retains its approved parameterized pgx bulk path. The deterministic engine remains free of database, HTTP and wall-clock dependencies.
- No frontend, explanation provider, LLM call, broker, generic repository, new library or analytics calibration change. Engine changes since Phase 02 are only configuration JSON tags, an explicit version and a copied configuration accessor.
- Twenty rounds of eight simultaneous authenticated analysis requests each produce exactly one new active run. Twenty rounds of competing claims produce one success and one `pgx.ErrNoRows`. The partial unique active-run index is retained. Atomic claims do not make startup recovery safe for multiple API processes; ADR-009's contrary wording was corrected.
- Existing tests prove wake-up, interrupted RUNNING recovery, queued-run preservation, bounded shutdown and a 200-ms injected timeout. New polling tests queue through a different service's wake channel and prove the same worker processes a valid run after an engine failure.
- Real PostgreSQL pause during analysis reaches a safe timeout failure after restoration, without partial results; the previous completed run remains current and a subsequent run completes. The real-process read-outage test verifies health 200, readiness 503, a bounded protected-request 503 with a matching request ID, and successful reads after restoration.
- The persistence rollback test fails after twelve meter statuses and four findings have been inserted, leaving no partial rows. API tests observe A as current across dashboard/meters/anomalies while B is queued, running and failed, then C as current after completion; A's historical run and findings remain available.
- Migration 00002 down preserves the seeded 12 meters, 4,032 readings, four events and consumption total; up and idempotent seed still work. The subsequent real-process PowerShell walkthrough analyzes the restored schema successfully. Direct invalid-state inserts are rejected for progress bounds, missing terminal timestamp/results, incomplete completion progress and missing failure error.
- Literal `%`, `_`, backslash and case-insensitive search are tested using synthetic database-only meter rows. Before analysis all status filters return zero; supplied-data statuses are 1 CRITICAL, 2 ALERT, 9 OK. Variation sorting uses absolute magnitude, with missing values last. Ranges are half-open; equal bounds are empty; one-sided bounds work. A synthetic 1,005-reading range returns exactly the default 1,000 rows with `has_more: true`.
- All four findings round-trip from engine evidence through JSONB to the API's typed evidence, schema_version 1. Source timestamps in episodes, signals, recovery and related events have no offset; run/session/row instants are RFC 3339 UTC. Filter intersections, historical IDs, priority order and zero-finding null confidence are covered.
- OpenAPI route, security, reference and DTO-field checks pass. Manual comparison covers enums, defaults, nullability, parameter bounds and time formats. Documentation now states media type/body limits, malformed/duplicate-query behavior and the readiness error's separate Health schema. These lightweight checks are not a general JSON Schema validator.

### Query And Runtime Evidence

The exact generated sqlc queries were run through `EXPLAIN (ANALYZE, BUFFERS)` after seeding a disposable PostgreSQL 18.6 database and completing analysis. Observed executions: reading range 0.086 ms (readings_pkey, 3 buffer hits); meter list 2.486 ms (one aggregate pass, 42 hits); current anomalies 0.211 ms (2 hits); latest completed 0.099 ms (1 hit). Tiny result/history tables correctly use sequential scans. These are local observations, not latency guarantees.

The PowerShell walkthrough completed migration 00002 down/up, idempotent seed and API/login/analyze/poll/anomalies/dashboard/logout with source data preserved. Bash curl requests independently completed the API/login/analysis/read/logout portion against that disposable database. The Linux black-box test separately verifies migrate/seed from a fresh database. README now separates server startup from second-terminal requests, provides native PowerShell JSON examples, uses the returned analysis ID, and pins the sqlc digest. Source CSV hashes still exactly match `data/input/README.md`.

### Remaining Limits

The approved limits remain: one API process; one configured demo credential; signed cookies without server-side revocation or login rate limiting; no automatic run retry; and per-request meter aggregation suitable for this dataset. If PostgreSQL remains unavailable beyond the five-second failure-recording attempt, restart recovery must fail the stranded RUNNING row after the database returns. There is no lease/reconciler. Source input is coherent during loading, but source data is not versioned across later reimports. CORS/same-origin frontend integration, explanation providers, metrics, CI and a single-command demo remain owned by later phases.

### Audited Change Inventory

All 68 changed paths are accounted for below (26 tracked changes, 42 new files). No unrelated changes or generated binaries are included. Local logs, coverage, query-plan output and temporary runtime helpers remain under ignored tmp directories.

| Category | Count | Files |
| --- | --- | --- |
| Shared governance | 3 | `.agents/context/project-context.md`, `.agents/skills/go-backend/SKILL.md`, `AGENTS.md` |
| Runtime/dependency/repository configuration | 3 | `.env.example`, `.gitignore`, `src/backend/go.mod` |
| Product documentation | 11 | `README.md`, `docs/adr/ADR-002-go-backend.md`, `docs/adr/ADR-007-background-analysis-execution.md`, `docs/architecture/architecture.md`, `docs/architecture/data-model.md`, `docs/phases/phase-03-backend-api.md`, `docs/phases/roadmap.md`, `docs/product/assumptions-and-decisions.md`, `docs/product/traceability-matrix.md`, `docs/testing/testing-strategy.md`, `docs/adr/ADR-009-durable-analysis-queue-in-postgresql.md` |
| Tests and integration fixtures | 18 | `src/backend/cmd/api/api_integration_test.go`, `src/backend/functional/blackbox_integration_test.go`, `src/backend/internal/config/config_test.go`, `src/backend/internal/platform/postgres/migrate_integration_test.go`, `src/backend/internal/platform/postgres/pgtest/pgtest.go`, `src/backend/functional/golden_path_integration_test.go`, `src/backend/internal/analysisrun/audit_integration_test.go`, `src/backend/internal/analysisrun/model_test.go`, `src/backend/internal/analysisrun/run_integration_test.go`, `src/backend/internal/auth/audit_test.go`, `src/backend/internal/auth/auth_test.go`, `src/backend/internal/httpapi/api_integration_test.go`, `src/backend/internal/httpapi/audit_integration_test.go`, `src/backend/internal/httpapi/audit_test.go`, `src/backend/internal/httpapi/openapi_test.go`, `src/backend/internal/httpapi/router_test.go`, `src/backend/internal/platform/jsontime/jsontime_test.go`, `src/backend/internal/platform/postgres/audit_integration_test.go` |
| Production Go (including removed router) | 20 | `src/backend/cmd/api/main.go`, `src/backend/cmd/api/router.go`, `src/backend/internal/analysis/config.go`, `src/backend/internal/analysis/engine.go`, `src/backend/internal/config/config.go`, `src/backend/internal/analysisrun/current.go`, `src/backend/internal/analysisrun/evidence.go`, `src/backend/internal/analysisrun/model.go`, `src/backend/internal/analysisrun/service.go`, `src/backend/internal/anomaly/anomaly.go`, `src/backend/internal/auth/auth.go`, `src/backend/internal/dashboard/dashboard.go`, `src/backend/internal/httpapi/dto.go`, `src/backend/internal/httpapi/errors.go`, `src/backend/internal/httpapi/middleware.go`, `src/backend/internal/httpapi/params.go`, `src/backend/internal/httpapi/router.go`, `src/backend/internal/meter/meter.go`, `src/backend/internal/platform/jsontime/jsontime.go`, `src/backend/internal/platform/postgres/snapshot.go` |
| Migrations, queries and sqlc configuration | 6 | `database/migrations/00002_create_analysis_results.sql`, `database/queries/analysis_runs.sql`, `database/queries/anomalies.sql`, `database/queries/dashboard.sql`, `database/queries/meters.sql`, `sqlc.yaml` |
| OpenAPI contract | 1 | `docs/api/openapi.yaml` |
| Generated sqlc (6 files) | 6 | `src/backend/internal/platform/postgres/dbgen/analysis_runs.sql.go`, `src/backend/internal/platform/postgres/dbgen/anomalies.sql.go`, `src/backend/internal/platform/postgres/dbgen/dashboard.sql.go`, `src/backend/internal/platform/postgres/dbgen/db.go`, `src/backend/internal/platform/postgres/dbgen/meters.sql.go`, `src/backend/internal/platform/postgres/dbgen/models.go` |

### Final Audit Validation

All production and test changes preceded these final validation runs. Counts distinguish top-level tests from subtests. No tests were disabled or skipped.

| Check | Executed command / result |
| --- | --- |
| Build and static analysis | `go -C src/backend build ./...`; `go -C src/backend vet -tags=integration ./...`: PASS. `gofmt -l` over repository Go files: empty. `git diff --check`: PASS |
| Modules | `go -C src/backend mod tidy`: no change to go.mod/go.sum hashes. `go -C src/backend mod verify`: all modules verified. The existing YAML dependency is direct for OpenAPI tests; no version change |
| Uncached unit | `go -C src/backend test -json -count=1 ./...`: 154 top-level tests + 89 subtests, zero failed/skipped |
| Shuffled unit | `go -C src/backend test -json -shuffle=on -count=1 ./...`: same counts, zero failed/skipped |
| Repeated unit | `go -C src/backend test -json -count=3 ./...`: 462 top-level executions + 267 subtest executions, zero failed/skipped |
| Uncached integration | `go -C src/backend test -json -tags=integration -count=1 ./...`: 199 top-level tests + 124 subtests, zero failed/skipped |
| Shuffled integration | `go -C src/backend test -json -tags=integration -shuffle=on -count=1 ./...`: same counts, zero failed/skipped |
| Final-tree race | In the Linux Go 1.27.1 + Docker CLI audit image: `go test -json -race -count=1 ./...` followed by `go test -json -race -count=1 -tags=integration ./...`: both PASS, 353 combined top-level executions + 213 subtest executions, zero failed/skipped, no race reports |
| sqlc reproducibility | Official `sqlc/sqlc:1.31.1`, resolved digest `sha256:70f53171d27b2424e9358869975455a6e955a5aa8e58a998a270a6e34e525537`. Repeated `generate` leaves all six generated file SHA-256 hashes identical; `diff` and `vet` exit 0. Only the claim query's generated changes are intentional |
| Coverage visibility | Combined Phase 03 capability coverage: **92.2%**. Commands below. Remaining gaps mainly cover unusual database-error paths, defensive/unreachable branches, entropy/serialization failure, malformed stored evidence and an error wrapper; no percentage target was imposed |
| Final black-box golden path | After every code/test fix and the complete race suites: `go test -json -tags=integration -count=1 ./functional -run TestBlackBox_AuthenticatedAnalysisGoldenPath` in Linux: PASS, 1 test, zero skips, 23.864 s test time |
| Data and cleanup | Source CSV hashes unchanged. Testcontainers cleaned up; the separately created query/runtime database was explicitly stopped and removed. Final `docker ps` empty. HEAD remains `6c92a6b`; index empty; no commit or push |

Coverage command (PowerShell; quote complete native arguments containing paths/commas):

```powershell
go -C src/backend test -tags=integration -count=1 '-coverpkg=./internal/auth,./internal/analysisrun,./internal/meter,./internal/anomaly,./internal/dashboard,./internal/httpapi,./internal/platform/jsontime' '-coverprofile=../../tmp/phase03-coverage.out' ./internal/auth ./internal/analysisrun ./internal/httpapi ./internal/platform/jsontime
go -C src/backend tool cover '-func=../../tmp/phase03-coverage.out'
```

Two initial coverage invocations failed at command setup because PowerShell split unquoted arguments; the fully quoted command above passed. This was an invocation problem, not a test failure or a reason to weaken tests.

For Linux validation, the existing temporary audit image combines `golang:1.27.1` with the official Docker CLI (image ID `sha256:614249a164ec0a6bcb6e9eb9c4f63fc1717f918bfc0f63084cd12e799a03e277`). The repository is mounted read-only at `/workspace`, the Go module cache at `/go/pkg/mod`, and the Docker socket at `/var/run/docker.sock`; `TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal`; working directory `/workspace/src/backend`. Build procedure: Phase 02 independent-audit evidence. No product Dockerfile was introduced.

The final golden path compiles migrate/seed/API, starts fresh PostgreSQL 18.6, migrates and seeds, verifies public health/readiness and unauthenticated 401, rejects invalid credentials, logs in with a real HTTP cookie jar, reads the session and pre-analysis dashboard, requests 202, polls to COMPLETED, reads all four findings, M-109 evidence and meter/readings state, verifies the dashboard, logs out and receives 401, then sends SIGTERM and requires process exit 0 with worker-stop/shutdown logs. Configured fake secrets and the issued cookie are absent from logs.

Final result: M-109 REAL_ANOMALY/HIGH/0.933333333333; M-112 DATA_QUALITY/HIGH/0.917295113841; M-104 EXPLAINABLE_ANOMALY/MEDIUM/0.728401058360; M-106 FALSE_POSITIVE/LOW/0.756360776550, in that priority order. CSV, PostgreSQL and API consumption totals agree at **155250.85 kWh**. Aggregate confidence is **0.8338475705209818**, the mean of those findings; high priority is **2**, derived from HIGH severity.

**Exit decision: COMPLETE WITH NON-BLOCKING NOTES.** No blocking Phase 03 defect remains. Actual tracked diff and every untracked file were reviewed with the code-review checklist. Build, regression, database, security, documentation and traceability gates pass. UI/design/Playwright acceptance is N/A because no frontend exists. **READY FOR PHASE 04** as a dependency assessment only; no phase is authorized next, and Phase 04 was NOT started. Recommended, unexecuted checkpoint: `feat: add persistent analysis API and orchestration`.

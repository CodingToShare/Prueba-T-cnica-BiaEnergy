# Testing Strategy

## 1. Objectives

Give fast, trustworthy evidence that the platform detects, classifies, prioritizes, and explains anomalies correctly, and that the product journey works, at every step of delivery.

**Testing is continuous, not deferred.** Every implementation phase adds the automated evidence for its own behavior and reruns the previously applicable regression suite. Phase 06 is quality hardening, complete regression, and delivery verification — it is not where testing starts.

Workflow for every phase: **Implement → Validate → Review → Document → Complete.** Never Implement → Mark Complete → Test Later.

## 2. Risk-Based Testing

Test effort follows risk and evaluation weight:

1. Analytics correctness: acceptance scenarios, M-109 detection and priority, M-106 not escalated, M-112 as data quality.
2. Evidence and explanation grounding.
3. Data integrity: exact ingestion, idempotency, constraints, time semantics.
4. API contracts and analysis-run lifecycle.
5. The critical product journey and UI states.

No repository-wide coverage percentage is set. Coverage reports may guide gap-finding but are never a goal.

## 3. Test Pyramid

Three complementary levels; none replaces another.

| Level | Purpose | Tools | Location |
| --- | --- | --- | --- |
| Unit | Isolated deterministic behavior, fast feedback | Go `testing` (+ testify where clearer); Vitest + Testing Library | Beside the code (`*_test.go`, `*.test.ts(x)`) |
| Integration | Real boundaries: PostgreSQL, HTTP + DB, ingestion | testcontainers-go + PostgreSQL | Beside the package, `*_integration_test.go` with the `integration` build tag (Go packages cannot live outside the module) |
| Functional (backend black-box) | Compiled commands as separate processes over real HTTP and PostgreSQL | Go `os/exec` + testcontainers (`integration` tag) | `src/backend/functional/` |
| Functional / E2E | Consumer-visible behavior through the real stack | Playwright | `tests/e2e/` |

Avoid duplicating the same assertion matrix across all levels: prove logic at unit level, prove wiring and data semantics at integration level, prove the journey at E2E level.

## 4. Unit Strategy

- **Analytics:** median, MAD and scaled spread floor, robust Z-score, percentage deviation, hour-aware baseline construction, persistence vs spike, multivariate and physical-consistency signals, event alignment and semantic compatibility, classification decisions, severity, confidence composition, priority ordering, evidence completeness, recommended-action coherence.
- **Backend:** input validation, error mapping to the standard error body, analysis-run state transitions, explanation provider selection and fallback.
- **Frontend:** formatting (units, percentages, dates), component behavior, filter/sort/search controls, important hooks, loading/empty/error states, accessibility roles and labels where practical.
- Do not unit-test framework internals or private implementation details.

## 5. PostgreSQL Integration Strategy

Real PostgreSQL via testcontainers-go whenever database semantics are under test; never mock SQL for these cases. Coverage: migrations from empty database, constraints (uniqueness, not-null, checks, foreign keys), CSV ingestion and idempotent reload, meter/reading/event persistence, time-range queries, analysis-run and anomaly persistence, evidence JSONB round-trips if used, server-side filtering/sorting. Each test owns its data (fresh schema or transaction) and is order-independent.

## 6. API Integration Strategy

Start the real HTTP handler stack against a real database. Verify status codes, response shapes against OpenAPI, validation errors, not-found, consistent error body without internals, filtering/sorting/pagination, analysis-run lifecycle including failure, health and readiness. Phase 03 also provides an API-level functional acceptance flow (trigger analysis → poll to completion → list anomalies in priority order → fetch M-109 detail with evidence) before any UI exists.

Integration tests run packages in parallel. On Windows, the shared `pgtest` helper sets `DOCKER_HOST` to the Docker SDK default when it is unset, because testcontainers' default Docker detection is unreliable under concurrency there (Phase 01 audit). The race detector needs cgo; on Windows without a C compiler, run `go test -race` in the official `golang` Linux container.

## 7. Frontend / Component Strategy

Vitest + Testing Library for component states (loading, empty, error with retry, not analyzed, disabled), interactions (filters, sort, search, run analysis), and accessibility basics (roles, labels, focus). Mock the network at the HTTP boundary only in component tests; E2E uses the real API.

## 8. Playwright Strategy

Real frontend → real API → real PostgreSQL in the main acceptance path; no mocked backend there.

- **Critical flow:** Login → Dashboard → Run AI Analysis → inspect anomalies → open M-109 → inspect evidence → understand explanation → see recommended action.
- **Additional high-value flows:** search/filter meters, meter detail navigation, anomaly filtering, analysis progress, recoverable API error, responsive navigation.
- Wait on observable conditions, never fixed sleeps. Keep specs focused by capability.

## 9. Analytics Acceptance Scenarios

Automated in Phase 02 through the real engine on the supplied data. Meter IDs appear only as test fixtures and assertions, never in production logic.

| Meter | Classification | Severity | Evidence the test should check where practical |
| --- | --- | --- | --- |
| M-104 | `EXPLAINABLE_ANOMALY` | `MEDIUM` | Persistent deviation; correlated compatible operational event |
| M-106 | `FALSE_POSITIVE` | `LOW` | Statistical deviation present, confined to a compatible event window — deviation ≠ real anomaly |
| M-109 | `REAL_ANOMALY` | `HIGH` | Strong consumption deviation; persistence; electrical-variable changes; no sufficiently explanatory event |
| M-112 | `DATA_QUALITY` | `HIGH` | Electrical inconsistency with consumption on profile |

Additional: M-109 ranks above all lower-priority findings; event correlation demonstrably changes classification (removing the M-106 event makes it no longer a false positive); perturbation test (renamed meters, shifted timestamps) yields identical classifications; control meters produce no escalated findings (calibration target).

## 10. LLM / Provider Testing Strategy

- The main CI pipeline never depends on a live remote or local LLM.
- The deterministic explanation provider is fully unit-tested per classification.
- `ExplanationProvider` contract tests run against every implementation.
- Ollama adapter tests use a local fake HTTP server: request shape, timeout, error, malformed and ungrounded output → fallback.
- Grounding validation tests reject invented numbers, events, or causes.
- Live Ollama validation is optional, manual or opt-in, and recorded separately.
- The product is verified correct with no LLM configured.

## 11. Regression Strategy

Every phase runs **focused tests for new/changed behavior + all previously applicable regression tests**. The Phase Entry Gate reruns the accepted baseline before new work (`docs/phases/roadmap.md`). A failing test is never hidden, skipped silently, or weakened to go green. Later phases may strengthen infrastructure but may not defer basic regression validation.

## 12. Deterministic Test Data

- Acceptance tests read the supplied CSVs from `data/input/` (read-only).
- Unit tests use small hand-built fixtures that isolate one behavior.
- Synthetic fixtures live only inside test code and are never written to `data/input/`.
- Time is injected (`Clock`); no randomness without a fixed seed; no dependence on test order or wall-clock time.
- `expected_results.csv` is never used.

## 13. Execution And Evidence Policy

- Concrete commands are documented as each phase establishes them; governance documents do not name commands that do not exist yet.
- Phase documents record the exact commands run, counts (passed/failed/skipped), and outcome after the final relevant change.
- Planned tests are not evidence. Skipped tests must be listed with a reason.
- An applicable test level may not be skipped for time pressure without recording the limitation.

## 14. Visual Validation

For significant screens, review or capture representative views at ~1440, ~768, and ~390 px and compare against `docs/design/reference/design-system-v3.html` per `docs/design/design-system.md` §9. Playwright screenshots may be evidence. No brittle pixel-perfect assertions unless a concrete regression justifies them.

## 15. Clean-Environment Validation

By Phase 06 the full suite and the documented startup (dependencies → migrate → seed → backend → frontend) are executed from a clean clone/environment, with results recorded. Integration tests already run on fresh containers from Phase 01.

## Current State

Phase 00 has no product tests (its validation is repository and documentation checks). Phase 01 established the first suites: unit tests (`go -C src/backend test ./...`) and integration, acceptance and functional tests against real PostgreSQL (`go -C src/backend test -tags=integration ./...`). Results are recorded in the Phase 01 document. Phase 02 added the analytics unit, synthetic-scenario and supplied-dataset acceptance tests in `internal/analysis`; they need no database and run in both commands. Results are recorded in the Phase 02 document.

Phase 03 added three kinds of tests:
- **Unit tests** (no Docker): demo authentication, source vs system time JSON, run-state helpers, evidence mapping, HTTP middleware, validation and error contract, and the OpenAPI checks (the document parses, references resolve, routes and schemas match the router and DTOs both ways).
- **PostgreSQL integration tests**: run lifecycle, failure and rollback, the concurrency guarantees, recovery, current-state consistency, and every API endpoint.
- **Black-box golden path**: login → analyze → poll → anomalies → meters → dashboard → logout, over compiled processes, real HTTP and cookies.

Results are recorded in the Phase 03 document.

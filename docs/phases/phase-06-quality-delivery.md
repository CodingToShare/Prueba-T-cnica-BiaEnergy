# Phase 06 — Quality + Observability + Delivery + Demo

- Status: **Complete** (independent final audit: ready with non-blocking notes; final local checkpoint created with the owner's separate authorization)
- Authorization: explicitly authorized by the user, together with the Phase 05 checkpoint commit; the final local checkpoint was authorized separately after the audit. Push, remote, tag, PR and release were not authorized by the phase; the first publication was authorized separately afterwards (see Remote Publication), and tag, PR and release remain unauthorized.
- Date: 2026-09-25
- Phase 05 checkpoint: local commit `dbfb945` "feat: add grounded AI explanation providers" (author Santiago Forero, `Phase 05:` body, no trailer; not pushed).

## Objective

Turn the complete product into a reproducible, reviewer-friendly submission that can be cloned, started with one command, demonstrated, tested and inspected. There is no product redesign and no new product feature.

## Phase 05 Checkpoint And Reconciliation

- **Phase 04 commit identity:** `699da3f` (HEAD before this phase) and `c258b46` have the same tree `780b9130…`, parent `2908b94` and author; `git diff 699da3f c258b46` is empty. They differ only in the message (`699da3f` adds the `Phase 04:` body requested by the owner) and the committer time. **`699da3f` is the effective Phase 04 checkpoint**; `c258b46` survives only in the reflog, and history was not rewritten.
- **Pre-existing governance edit:** the one-line commit-message-format rule in `.agents/skills/phase-governance/SKILL.md` was written at the owner's request after Phase 05 and is not Phase 05 behavior (case B). It was left unstaged and untouched, excluded from the Phase 05 commit, and is excluded from Phase 06 ownership in every diff review.
- **Phase 05 change set:** 64 paths staged explicitly with a pathspec that excludes the skill file. No binaries, models, `.env`, artifacts or Phase 06 work.
- **Pre-commit regression:** `mod verify` and `vet` clean; unit 183 top-level + 117 subtests; frontend lint, typecheck, 14 files / 81 tests and build; focused deterministic, fallback and black-box fallback tests; `git diff --check` clean.
  - The first integration run had one failure: `TestAPI_HealthReadinessAndGracefulShutdown` ("did not shut down within 15s") while a frontend build and all integration packages ran in parallel (the container took 10 s to create). It then passed 3/3 in isolation, and a clean full rerun passed 237 + 157 with 0 failures and 0 skips. Committed as `dbfb945`.
  - The root cause was investigated in this phase; see Hardening.

## Phase Entry Gate

| Check | Result |
| --- | --- |
| Previous phase | Phase 05 committed (`dbfb945`) after audit (complete with non-blocking notes) |
| Baseline | The regression above on the same tree; product behavior stable |
| Context read | AGENTS.md, project context, phase-governance, testing, code-review, documentation and observability skills, this contract, roadmap, architecture, testing strategy, Definition of Done, performance strategy, OpenAPI, README, out-of-scope, traceability |
| Scope | Delivery hardening only; the analytics engine, thresholds and product features are unchanged |

## Implementation

| Area | What was built |
| --- | --- |
| Metrics (TD-31) | `internal/platform/metrics`: Prometheus client 1.24.1 with an application-owned registry (no global state) and nil-safe methods. HTTP counters and durations by method, route template and status class (unmatched paths → `unmatched`, unknown methods → `OTHER`); analysis runs by terminal status (counted after the result transaction commits), run duration, findings per run, active runs; explanation outcomes (`generated`/`fallback`/`unavailable`) by provider, explanation duration, fallbacks by sanitized code; Go runtime and process collectors. `GET /metrics` is public like `/healthz` and `/readyz` and documented in the OpenAPI operations section |
| Backend image | `src/backend/Dockerfile`: `golang:1.27.1-bookworm` build (`CGO_ENABLED=0`, `-trimpath`) → `gcr.io/distroless/static-debian12:nonroot`. One image with `api` (default), `migrate`, `seed` and the new `healthcheck` probe (standard library only), plus the migrations and supplied CSVs |
| Frontend image | `src/frontend/Dockerfile`: `node:24.21.0-alpine`, pnpm 12.6.0, frozen install, Next.js standalone build (enabled only by `NEXT_OUTPUT=standalone` in the image build, so `next start` is unchanged), runtime of traced files and static assets as user `node`, with the base image's npm, npx, yarn and corepack removed (only `node` remains). `BACKEND_URL=http://api:8080` is a build argument |
| Compose delivery (OD-19) | One `compose.yaml`: `postgres` (healthy) → `migrate` (completes) → `seed` (completes) → `api` (healthy via `healthcheck` on `/readyz`) → `web` (healthy); `init: true` for Node; local-demo defaults overridable by environment/`.env`; ports on 127.0.0.1; `host.docker.internal:host-gateway` for optional host Ollama; `docker compose up -d --wait postgres` keeps the database-only developer workflow |
| Reset | `docker compose down -v --remove-orphans` then the start command; only this project's containers, network and volume |
| CI | `.github/workflows/ci.yml`, five jobs:<br>• backend: mod verify, `mod tidy -diff`, gofmt, vet ×2, build, unit tests, pinned sqlc generate/vet + `git diff --exit-code`<br>• frontend: frozen install, types regeneration + drift check, lint, typecheck, tests, build<br>• integration: Testcontainers<br>• e2e: `pnpm test:e2e` with Chromium; artifacts on failure<br>• delivery: Compose start, containerized golden path + axe, metrics check, logs on failure<br>Go from `go.mod`, Node 24.21.0, pnpm from `packageManager`; no secrets, no model, no schedule, no deployment |
| Accessibility | `@axe-core/playwright` 4.13.0 in `e2e/accessibility.spec.ts` (project `a11y`): WCAG 2.1 A/AA scan of login, dashboard, meters, meter detail, anomalies, investigation (1440 px) and dashboard, investigation (390 px). Serious and critical violations fail; the rest is written per page; each scan must evaluate more than 10 rules |
| Hardening | Pool connection attempts are bounded by the existing 5 s connect timeout (unless the URL sets `connect_timeout`). Plausible cause of the shutdown flake: pgx keeps building a connection in the background after the caller's context is cancelled, and `pool.Close()` waits for it; a dial to a stopped container can hang until the OS TCP timeout. The flake did not reproduce (6/6 under comparable load) |
| Environment contract | `.env.example` rewritten: Compose needs no `.env`; one set of local-demo values (`demo` / `bia-demo-2026`, a signing key labelled not-for-production); host vs container Ollama URL explained |
| Documentation | README rewritten reviewer-first; demo guide with technical talking points; submission checklist; architecture §9 (metric names) and §13 (delivery); final test matrix; decisions (TD-31, TD-32, OD-19 resolved, Phase 06 resolutions); traceability; AGENTS delivery section; project context |

Not added (overengineering review): Kubernetes, Helm, Terraform, Argo, cloud deployment, service mesh, Kafka, Redis, Grafana, Loki, Tempo, ELK, Vault, an OAuth provider, `act`, wrapper scripts or task runners. The analytics engine, its thresholds and every product feature are unchanged, and no database migration was added.

## Validation Evidence (final tree)

| Gate | Command | Result |
| --- | --- | --- |
| Formatting / modules | gofmt (tracked + new Go files), `go mod tidy -diff`, `go mod verify` | Clean / all verified |
| Build / static | `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...` | Clean |
| sqlc | pinned image `generate` ×2, `vet`, `diff`; `dbgen` unchanged | Clean |
| Backend unit | `go -C src/backend test -json -count=1 ./...` | **191 + 117** passed, 0 failed, 0 skipped |
| Backend integration | `go -C src/backend test -json -count=1 -tags=integration ./...` | **246 + 157** passed, 0 failed, 0 skipped (PostgreSQL 18.6) |
| Race (Linux `golang:1.27.1` + Docker CLI, repo read-only) | `go test -race -count=1` normal and `-tags=integration` | 191 + 117 and 246 + 157 passed; **0** data races |
| Frontend | `pnpm install --frozen-lockfile`, `pnpm generate:api` ×2, `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` | Identical generations; clean; **14 files / 81 tests**; build OK |
| Dependency audit | `pnpm audit`; `go mod verify` | No known vulnerabilities; verified |
| Real-stack E2E | `pnpm test:e2e` | **11/11** (desktop 2, audit 6, a11y 1, mobile 1, tablet 1) + API outage/recovery + Ollama-unavailable fallback; containers removed |
| Images | `docker compose build --no-cache`, then `docker compose build` | 68 s, then 3 s (first run); final tree after the runtime hardening: `--no-cache` 71 s. `bia-energy-backend:local` 66.2 MB, `bia-energy-web:local` 298 MB (removing the package managers does not shrink the base layers) |
| Links | one-off relative-link check of all 58 Markdown files (no repository tool exists) | 43 links, 0 broken after fixing one stale anchor |
| `git diff --check` | | Clean |

The generated-types drift check caught a real miss: `/metrics` had been added to the OpenAPI without regenerating the frontend types. The types were regenerated (stable across two runs); CI now enforces this.

**Coverage (visibility only, no threshold).** Backend unit + integration, `-coverpkg=./internal/...,./cmd/...`: **90.9%** of statements. Examples:

| Package | Coverage |
| --- | --- |
| analysis | 98.1% |
| analysisrun | 90.7% |
| explanation | 91.1% |
| httpapi | 97.5% |
| ingestion | 97.7% |
| metrics | 99.3% |
| postgres | 82.9% |

The `cmd/*` packages run mostly as separate processes in the black-box tests and are not instrumented; `cmd/healthcheck` is exercised by every container health probe. The frontend has no coverage figure: Vitest's coverage provider is not installed, and it was not added just to produce a number.

## Delivery Validation (containers only)

- **Clean state:** the project's containers, volume and images were removed, along with `.next` and `test-results`; the images were rebuilt with `--no-cache`.
- **README rehearsal in PowerShell,** following the README literally:
  - `docker compose up --build -d --wait` → exit 0 in 23 s. `migrate` and `seed` exited 0; `postgres`, `api` and `web` are healthy.
  - The documented containerized smoke (`E2E_BASE_URL=http://127.0.0.1:3000`, `demo` / `bia-demo-2026`, `--project desktop --project a11y`) → 3/3 in 20.6 s. It covers invalid login, the golden path (pre-analysis honesty, Run AI Analysis, 4 findings / 2 high, M-109 investigation with "Evidence-based explanation", meters and chart, logout) and axe.
  - `/metrics`: `bia_analysis_runs_total{status="completed"} 1`, `bia_analysis_findings_count 1` (one completed-run observation), `bia_analysis_findings_sum 4` (four findings in that run), `bia_explanation_generation_total{outcome="generated",provider="deterministic"} 4`.
  - Documented reset → fresh database (12 meters, 4,032 readings, `latest_analysis` null, analysis ID back to 1) → analysis COMPLETED with 4 findings / 2 high in 93 ms.
  - The same flow also ran from Bash. The mobile journey against the containers passed, and the screenshots show the loaded font, charts, responsive shell and explanation panel.
- **Shutdown:** `docker compose down` takes 2 s and keeps the volume. `stop api` logs "shutdown started" → "analysis worker stopped" → "shutdown complete" with exit 0. `web` exits 143 on SIGTERM within 1 s (no SIGKILL).
- **Container review:**
  - The backend runs as `nonroot:nonroot` on distroless (no shell or package manager); the web runs as `node` with npm, npx, yarn and corepack removed from the runtime (a final-review fix). Neither is privileged, and neither mounts the Docker socket.
  - Only 5432/8080/3000 are published, on 127.0.0.1.
  - The images contain no `.git`, host `node_modules`, tests, e2e, `.env`, docs or model, and the web runtime has no configuration variables. The browser chunks contain neither `api:8080` nor any `BACKEND_URL`.
  - Build contexts are limited by `.dockerignore` files.
- **Optional Ollama from Docker (manual, validated):**
  - Setup: Docker Desktop for Windows, host Ollama 0.34.4 on its default `127.0.0.1:11434`, `EXPLANATION_PROVIDER=ollama OLLAMA_MODEL=llama3.2:3b docker compose up -d --wait`.
  - Result: 4/4 explanations `OLLAMA` from `llama3.2:3b`, no fallback, analysis in 40 s; `bia_explanation_generation_total{outcome="generated",provider="ollama"} 4`.
  - Not validated: Linux hosts (`host-gateway` mapping configured but untested).
- **Fallback metric (containers):** with `OLLAMA_BASE_URL=http://127.0.0.1:1`, the analysis COMPLETED with 4 findings; `bia_explanation_fallback_total{fallback_code="provider_unavailable"} 4`, `bia_explanation_generation_total{outcome="fallback",provider="ollama"} 4`, and no identifier in any label.

## Final Review Pass

- **Diff review:** only intended Phase 06 files; the pre-existing governance edit stays excluded. No secrets, binaries, runtime state, traces, coverage, screenshots, `.env`, TODOs or debug output. The generated sources (sqlc `dbgen`, OpenAPI types) regenerate identically.
- **Hardening fix from this pass:** the web runtime still contained the Node image's npm, npx and yarn (found by listing the exported image). They are removed in the runtime stage.
- **Revalidation on the final images:** `--no-cache` build; one-command start (exit 0, 24 s, `migrate`/`seed` exited 0, the others healthy); containerized golden path and axe 3/3 with 0 violations on all 8 scans; metrics; documented reset (7 project resources removed, no project volume left, restart with 12 meters, 4,032 readings, 0 anomalies, `latest_analysis` null); `stop` (API exit 0 after "analysis worker stopped" and "shutdown complete", web exit 143 on SIGTERM); `down` leaves no project containers.
- **Image and bundle review:** neither image's environment contains configuration values. Exported images contain none of the demo password, signing key or database password. `api:8080` appears only in the web server's `routes-manifest.json`, `required-server-files.json` and `server.js`, never in `.next/static`. The backend image holds only `/app/bin` (4 binaries), the migrations and the two CSVs.
- **Source data:** SHA-256 readings `01c953c2daa6503f9696a46096f34c635d67b7394054ad3f628a0230393b162f`, events `750d42f11c2c409af7094153fd07071d3aa3b5a8d68bb418c9fcaf7cbb0c9932`, identical to the Phase 01 record. No `expected_results` file exists anywhere in the working tree.

## Independent Final Cross-Phase Audit

The independent audit on 2026-09-25 treated every earlier report as evidence to reproduce. Committed HEAD remained Phase 05 checkpoint `dbfb945`; Phase 06 remained unstaged and uncommitted, and the unrelated `.agents/skills/phase-governance/SKILL.md` edit remained untouched and excluded.

- **Complete regression:** 308 backend unit test cases and 403 integration/black-box test cases passed with no failures or skips; both suites passed again under the Linux race detector with no reported race. Frontend lint, typecheck, 14 files / 81 tests, production build and production dependency audit passed. The real-stack Playwright suite passed 11/11 plus API recovery and Ollama-unavailable fallback.
- **Analytics and authority:** focused no-future-leakage, synthetic stable/anomalous/data-quality/spike, unknown-event, rename/time-shift, threshold-sensitivity, shuffle/determinism and supplied-data acceptance tests passed. The final deterministic result remained exactly four findings in order: M-109, M-112, M-104, M-106. Focused injection, malformed-output, unsupported-consequence, action-grounding and cross-metric-number rejection tests passed.
- **Generators and contracts:** OpenAPI types and sqlc output each regenerated twice identically; sqlc vet, route/schema/security drift tests and YAML parsing passed. Forty-four relative documentation links resolved.
- **README-only delivery:** after `docker compose down -v --remove-orphans`, `docker compose up --build -d --wait` reached healthy in 30.9 s; migrate and seed exited 0. The documented container smoke passed 3/3, including the real browser golden path and axe. The documented reset removed only the `bia-energy` project resources, restarted to 12 meters / 4,032 readings / no analysis / no anomalies / null latest analysis, then analysis ID 1 completed with four findings and two high-priority findings.
- **Metrics corrections:** the audit made the findings Histogram wording and unit assertions explicit: the live completed run exposed `_count 1` (one run observation) and `_sum 4` (four findings). It also corrected a failure edge case so a worker whose FAILED status cannot be stored clears the active gauge without incrementing a terminal-status counter. All application labels remained bounded and identifier-free.
- **Images and shutdown:** backend 66,161,473 bytes as `nonroot:nonroot`, containing four binaries, three migrations and two source CSVs under `/app`; web 297,983,953 bytes as `node`, with a 41.9 MB standalone application and no source/tests/Playwright/pnpm store or package-manager executable. API stopped gracefully with exit 0 and the expected worker/shutdown log sequence; the Node server accepted SIGTERM with exit 143. Final cleanup left zero project containers and zero project volumes.
- **Integrity:** source SHA-256 values matched Phase 01, production code contained no canonical meter IDs or forced four-result path, no evaluator-result CSV filename was present, and `git diff --check` was clean. No blocking or important defect remained.

## Real Dataset Result (containers, deterministic mode)

| Priority | Meter | Type | Severity | Confidence |
| --- | --- | --- | --- | --- |
| 1 | M-109 | REAL_ANOMALY | HIGH | 0.9333333333333335 |
| 2 | M-112 | DATA_QUALITY | HIGH | 0.9172951138412555 |
| 3 | M-104 | EXPLAINABLE_ANOMALY | MEDIUM | 0.7284010583598136 |
| 4 | M-106 | FALSE_POSITIVE | LOW | 0.7563607765495244 |

These are identical to Phase 02–05, with 155,250.85 kWh for 12 meters and 4,032 readings.

## Performance Sanity (local, containers; not a benchmark)

| Measurement | Value |
| --- | --- |
| Analysis, end to end through the proxy | 93–282 ms, including polling. Run duration from the `bia_analysis_run_duration_seconds` sum: 0.078 s |
| Dashboard summary / meter list / anomaly list, through the web proxy | 28 / 17 / 7 ms |
| Web container start → healthy | 9.6 s (Next.js ready immediately; the rest is the health-check cadence) |
| One-command start with built images | 23–24 s; first `--no-cache` image build 68 s |
| Deterministic explanation | about 2.2 µs per finding (Phase 05 benchmark) |
| Live Ollama | 6.4–10.5 s per finding warm (Phase 05 records; 40 s for four from Docker) |

## Security, Artifacts And Integrity

- **Secrets:** the diff and repository scan found no private keys, tokens, real passwords, cookies, dumps or model files. The only credential-like values are the clearly labelled local-demo defaults. `.env` stays ignored.
- **Dependencies:** the only new dependencies are `github.com/prometheus/client_golang` 1.24.1 (with its transitive modules) and the dev dependency `@axe-core/playwright` 4.13.0.
- **Source data:** CSV blob hashes are unchanged: readings `1c1a11a7aef21a75e4649d8e71f52747b1cf8816`, events `d6a23acd4b6fbb0600f4fa835fe46bbd9944e426`. No `expected_results` file exists. `git grep` finds no meter IDs or challenge event descriptions in production code.
- **Generated artifacts:** none in the diff (`.next`, `node_modules`, `test-results`, coverage, binaries and volumes are ignored or outside the repository). Intentionally generated sources remain: sqlc `dbgen` and the OpenAPI TypeScript types.

## Phase Exit Gate

| Check | Result |
| --- | --- |
| Scope (metrics, images, Compose, one-command start and reset, CI, axe, docs, demo guide, checklist) | Done |
| Product preserved (analytics, deterministic default, optional Ollama) | Verified |
| Tests with the implementation (metrics unit/HTTP/integration, axe, delivery smoke) | Done; all green |
| Regression Phases 01–05 | Backend unit, integration and race; frontend; E2E: all green |
| Clean-environment and README rehearsal | Done (Windows host: PowerShell and Bash) |
| CI | Configured; every constituent command validated locally; **no job has started on GitHub** (account billing lock, see Remote Publication) |
| Docs and traceability | Updated; the only open row is FR-DEL-001's live demo presentation by the owner |
| Commit | Final local checkpoint `dbc3a8b`, created with the owner's separate authorization after the audit |

## Remote Publication

With the owner's separate authorization, the repository was published after the Phase 06 checkpoint and the governance commit `3ea7814`:

- Repository: https://github.com/CodingToShare/Prueba-T-cnica-BiaEnergy (public, default branch `main`). The local branch was renamed from `master` to `main`, and one push without force created `main` at `3ea7814`, tracking `origin/main`. No tag, release or pull request was created.
- Before the push, a review of all 276 tracked files found no `.env`, credentials, build output, test artifacts, database files or model files; the only env files are the two `.env.example` templates with local-demo values.
- GitHub Actions run `36208094299` (workflow `ci`, event push, head `3ea7814`) created all five jobs, and GitHub refused to start each one: "The job was not started because your account is locked due to a billing issue." No runner was assigned and zero steps executed. This is an account/infrastructure block, not a failing test or workflow step, and CI is **not** reported as passing. The remediation is to clear the billing lock and rerun; no code or workflow change is indicated.

## Known Limitations And Deferred Work

- No GitHub Actions job has executed yet (account billing lock); remote CI remains to be validated once billing is resolved.
- The 5–10 minute demo is presented by the owner at submission; the guide and a timed rehearsal of the flow exist.
- Clean-room validation ran on this Windows machine (with Docker's module/pnpm cache mounts available), not on a separate clean machine; Linux was exercised only through the containers and the race run.
- Optional Ollama from Docker was validated on Docker Desktop for Windows only.
- Demo-grade security, as already documented: a single demo user, unauthenticated operational endpoints on localhost, plain HTTP, `BACKEND_URL` fixed at image build time.
- No dashboards or alerting (metrics endpoint only, by design).

## Next Phase

None. The roadmap ends at Phase 06; any further push, CI rerun, tag, release or pull request requires explicit owner authorization.

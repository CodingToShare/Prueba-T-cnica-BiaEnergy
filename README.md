# Bia Energy Management Platform

An MVP for electrical meter management that turns hourly meter data into operational decisions. It detects anomalies, classifies them as real, operationally explainable, false positive or data quality, explains them with evidence, prioritizes what to investigate first, and recommends an action.

Built for a three-day Full Stack + Data + AI challenge. Success cycle: `DATA → ANALYSIS → ANOMALY → EXPLANATION → PRIORITIZATION → ACTION`.

## Architecture At A Glance

```text
Browser ──► Next.js (UI + same-origin /api/v1 proxy) ──► Go REST API ──► PostgreSQL 18
                                                            │   ▲
                                                            ▼   │ results + explanations
                                                 background analysis worker
                                                            │
                                         deterministic anomaly engine (pure Go)
                                                            │
                                         ExplanationProvider ── deterministic (default)
                                                            └── Ollama* (optional, local)
   Operations: /healthz  /readyz  /metrics (Prometheus)          * never required
```

A pragmatic modular monolith (ADR-001). Detection, classification, severity, confidence and priority are deterministic statistics: robust hour-of-day baselines (median/MAD), persistence, multivariate checks and event correlation. The optional language model only rewords evidence that has already been computed, and a failure falls back to deterministic text. Details: [architecture](docs/architecture/architecture.md), [anomaly analysis](docs/ai/anomaly-analysis.md), [explainability](docs/ai/explainability.md), [ADRs](docs/adr/).

## Prerequisites

- **Demo:** Docker with Compose v2 (Docker Desktop on Windows/macOS, or Docker Engine + the Compose plugin on Linux). Nothing else: no Go, Node or model download.
- **Manual development and tests:** Go 1.27.x, Node.js 24, pnpm 12 (see [environment readiness](docs/quality/environment-readiness.md)).

## Quick Start (One Command)

From the repository root, in PowerShell, Bash or any shell:

```sh
docker compose up --build -d --wait
```

This builds the two images, then starts the stack in order: PostgreSQL 18.6 → migrations → import of the supplied CSVs → Go API → Next.js. It returns when everything is healthy (about a minute on the first build, seconds afterwards). No `.env` file is needed.

Open **http://localhost:3000** and sign in with the local demo credentials:

| Username | Password |
| --- | --- |
| `demo` | `bia-demo-2026` |

These are **local-demo defaults** set in `compose.yaml`. They are not secrets and not production credentials; override them with environment variables or a `.env` file (see [.env.example](.env.example)).

## Run The Analysis

1. The dashboard first shows the source data only: 12 meters, 4,032 hourly readings, 155,250.85 kWh, and "Not analyzed yet".
2. Click **Run AI Analysis**. The deterministic engine finishes in well under a second, and the status panel reports the result.
3. Open the investigation queue, **AI Anomalies** or **Meters** to explore each finding, its evidence, the reading chart and the recommended action.

Expected result on the supplied data (demo material; the product has no meter-specific logic):

| Priority | Meter | Classification | Severity | Meaning |
| --- | --- | --- | --- | --- |
| 1 | M-109 | Real anomaly | High | Sustained consumption rise with current and power-factor changes; no event explains it |
| 2 | M-112 | Data quality | High | Voltage/current/power factor inconsistent while consumption stays normal |
| 3 | M-104 | Explainable anomaly | Medium | Real rise explained by a recorded operational change |
| 4 | M-106 | False positive | Low | Drop explained by a scheduled outage, followed by recovery |

A 5–10 minute walkthrough is in the [demo guide](docs/demo/demo-guide.md).

## Stop, Restart, Reset

```sh
docker compose down                        # stop; the database volume (and analyses) is kept
docker compose up --build -d --wait        # start again
docker compose down -v --remove-orphans    # RESET: also deletes this project's database volume
```

After a reset, the next `docker compose up --build -d --wait` recreates a fresh database, migrates it and imports the challenge data again, with no analysis. Only this project's containers, network and `postgres-data` volume are touched.

## Optional: Local Ollama Explanations

The default demo uses deterministic explanations (`EXPLANATION_PROVIDER=deterministic`): no model, predictable latency. To try generated explanations with a model on your machine:

```sh
ollama pull llama3.2:3b     # about 2.0 GB, once; the model used for the recorded validation
```

```powershell
$env:EXPLANATION_PROVIDER = "ollama"; $env:OLLAMA_MODEL = "llama3.2:3b"; docker compose up -d --wait   # PowerShell
```

```sh
EXPLANATION_PROVIDER=ollama OLLAMA_MODEL=llama3.2:3b docker compose up -d --wait   # Bash
```

The API container reaches Ollama on the host at `http://host.docker.internal:11434` (validated on Docker Desktop for Windows with Ollama on its default `127.0.0.1:11434`; other platforms were not validated). The model only rewords the evidence: classification, severity, confidence, priority and the action code always come from the engine. If Ollama is unavailable, slow or returns invalid output, that finding gets the deterministic text (`fallback_used: true`) and the analysis still completes. Expect about 6–10 s per finding once the model is loaded (4 GiB GPU); loading a cold model takes longer. To return to the default, unset the variables and run the command again.

## Operations

| Endpoint (API, `http://localhost:8080`) | Purpose |
| --- | --- |
| `GET /healthz` | Process alive |
| `GET /readyz` | PostgreSQL reachable (503 otherwise) |
| `GET /metrics` | Prometheus text format: HTTP requests by route template, analysis runs/duration/findings, explanation outcomes and fallbacks, Go runtime |

The published ports are bound to `127.0.0.1`. The operational endpoints are unauthenticated for local use; a real deployment restricts them at the network edge. Logs are structured JSON: `docker compose logs api`.

## Manual Development

Run the Go commands and Next.js on the host against the Compose database. Values are the same local-demo defaults (see `.env.example`). Commands run from the repository root; `go -C src/backend` runs Go inside the backend module.

1. Start only PostgreSQL (localhost:5432):

   ```sh
   docker compose up -d --wait postgres
   ```

2. Set the environment:

   ```powershell
   $env:DATABASE_URL = "postgres://bia:bia_local_dev@localhost:5432/bia_energy?sslmode=disable"   # PowerShell
   $env:DEMO_AUTH_USERNAME = "demo"; $env:DEMO_AUTH_PASSWORD = "bia-demo-2026"
   $env:SESSION_SIGNING_KEY = "local-demo-only-signing-key-not-for-production"; $env:SESSION_COOKIE_SECURE = "false"
   ```

   ```sh
   export DATABASE_URL="postgres://bia:bia_local_dev@localhost:5432/bia_energy?sslmode=disable"    # Bash/zsh
   export DEMO_AUTH_USERNAME=demo DEMO_AUTH_PASSWORD=bia-demo-2026
   export SESSION_SIGNING_KEY=local-demo-only-signing-key-not-for-production SESSION_COOKIE_SECURE=false
   ```

3. Migrate, import the dataset (both idempotent) and start the API with its background worker:

   ```sh
   go -C src/backend run ./cmd/migrate up      # also: status, down
   go -C src/backend run ./cmd/seed
   go -C src/backend run ./cmd/api             # :8080; LOG_LEVEL=debug for more output
   ```

   Optional Ollama for a host API: `EXPLANATION_PROVIDER=ollama`, `OLLAMA_MODEL=llama3.2:3b` (base URL defaults to `http://127.0.0.1:11434`).

4. In another terminal, start the frontend. It forwards `/api/v1/*` to the server-only `BACKEND_URL` (default `http://localhost:8080`, read at `pnpm dev` startup or at `pnpm build` time):

   ```sh
   cd src/frontend
   pnpm install --frozen-lockfile
   pnpm dev                                    # http://localhost:3000
   ```

   Do not run the Compose `api`/`web` services at the same time: they use the same ports. The API contract is [docs/api/openapi.yaml](docs/api/openapi.yaml); the schema is described in the [data model](docs/architecture/data-model.md). After changing `database/queries` or a migration, regenerate the typed queries with the pinned image:

   ```sh
   docker run --rm -v "${PWD}:/src" -w /src sqlc/sqlc:1.31.1@sha256:70f53171d27b2424e9358869975455a6e955a5aa8e58a998a270a6e34e525537 generate   # PowerShell; Bash: "$(pwd):/src" (Git Bash: prefix MSYS_NO_PATHCONV=1)
   ```

## Tests

| Suite | Command | Needs |
| --- | --- | --- |
| Backend unit + analytics acceptance | `go -C src/backend test ./...` | Go |
| Backend integration + black-box (PostgreSQL 18.6) | `go -C src/backend test -tags=integration ./...` | Go, Docker |
| Frontend lint / types / unit + component | `pnpm lint`, `pnpm typecheck`, `pnpm test` (in `src/frontend`) | Node, pnpm |
| Real-stack E2E: golden path, audits, responsive, accessibility (axe), API recovery, Ollama fallback | `pnpm exec playwright install chromium` once, then `pnpm test:e2e` (in `src/frontend`) | Go, Node, Docker |
| Containerized smoke against the Compose demo (fresh database) | from `src/frontend`: `E2E_BASE_URL=http://127.0.0.1:3000 E2E_USERNAME=demo E2E_PASSWORD=bia-demo-2026 pnpm exec playwright test --project desktop --project a11y` (PowerShell: set the three `$env:` variables first) | Running demo stack |

`pnpm test:e2e` starts its own disposable PostgreSQL container, compiled API and production frontend on free ports and removes them afterwards; it never touches the Compose database. No test needs a language model. CI runs these suites plus generated-code drift checks and the Compose delivery path ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)). Full matrix: [testing strategy](docs/testing/testing-strategy.md).

## Key Engineering Decisions

- **Deterministic analytics, optional generative explanation.** The LLM can never change a classification, severity, confidence, priority or action (ADR-004, ADR-006).
- **Robust statistics per meter and hour of day.** Median/MAD, a dual relative + robust-Z gate, persistence and recovery, and event roles (explains / corroborates / context).
- **PostgreSQL as source of truth and queue.** Analysis runs are persisted and claimed with `FOR UPDATE SKIP LOCKED`, one active at a time; the product always shows the latest completed run (ADR-009).
- **sqlc over an ORM, `NUMERIC` measurements, timezone-naive source times** (TD-17, TD-18, ADR-008).
- **Signed HttpOnly session cookie behind a same-origin Next.js proxy.** No CORS and no tokens in the browser (OD-12, TD-27).
- **Reviewer-first delivery.** One Compose file, one command, deterministic by default, low-cardinality Prometheus metrics.

Interview-style rationale for these decisions: [demo guide § Technical talking points](docs/demo/demo-guide.md#technical-talking-points).

## Known Limitations

- Demo-grade authentication: one configured user, no user store, no session revocation before expiry, no rate limiting.
- The frontend's `BACKEND_URL` is fixed at image build time (Next.js rewrites); the Compose build targets the `api` service.
- Operational endpoints are unauthenticated; plain HTTP on localhost (`SESSION_COOKIE_SECURE=false` only for this reason).
- Optional Ollama explanations vary by model and run. Grounding checks reject many failures but cannot prove every phrase; the structured evidence remains the reference.
- Single API process by design (ADR-009); no horizontal scaling, dashboards or alerting are provided.
- The CI workflow is configured and its commands are validated locally; it has not run on GitHub because this repository has not been pushed.

## Repository And Documentation

```text
compose.yaml        demo stack (PostgreSQL, migrate, seed, API, web)
.github/workflows/  CI
data/input/         supplied challenge CSVs (unchanged)
database/           migrations (goose) and queries (sqlc)
src/backend/        Go module: api, migrate, seed, healthcheck commands; Dockerfile
src/frontend/       Next.js app, Playwright journeys (e2e/); Dockerfile
docs/               product, architecture, ADRs, AI, design, testing, quality, phases, demo, delivery
AGENTS.md           engineering rules for contributors and coding agents
```

Product: [business context](docs/product/business-context.md), [requirements](docs/product/functional-requirements.md), [business rules](docs/product/business-rules.md), [traceability](docs/product/traceability-matrix.md). Delivery: [submission checklist](docs/delivery/submission-checklist.md). Phases and evidence: [roadmap](docs/phases/roadmap.md). The evaluator's ground-truth file is not part of this repository and is never used.

## Contributing

Read [AGENTS.md](AGENTS.md) first. Quality gate: [Definition of Done](docs/quality/definition-of-done.md).

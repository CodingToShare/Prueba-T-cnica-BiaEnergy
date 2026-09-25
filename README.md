# Bia Energy Management Platform

An MVP for electrical meter management that turns energy data into operational decisions: it detects anomalies, explains them with evidence, prioritizes what to investigate first, and recommends an action.

> **Current status:** Phase 02 is complete. The Go runtime, the PostgreSQL schema, the verified, idempotent import of the challenge dataset, and health and readiness endpoints work. The deterministic anomaly engine (`src/backend/internal/analysis`) classifies, prioritizes and explains the four challenge scenarios with evidence, verified by automated tests; it is not exposed yet. **The product API, analysis runs and the frontend are not implemented yet** (Phases 03–05).

## Challenge Summary

Full Stack + Data + AI technical challenge with a three-day delivery window. The platform must answer:

- What is happening with the meters?
- Which readings deviate from expected behavior?
- Is an anomaly real, operationally explainable, a false positive, or a data-quality problem?
- Which should be investigated first, why, and with what evidence?
- What action is recommended?

Success cycle: `DATA → ANALYSIS → ANOMALY → EXPLANATION → PRIORITIZATION → ACTION`.

Product flow: `Login → Dashboard → Meters → Meter Detail → Run AI Analysis → AI Anomalies → Investigation → Recommended Action`.

Details: [business context](docs/product/business-context.md), [functional requirements](docs/product/functional-requirements.md), [business rules](docs/product/business-rules.md).

## Architecture Summary

A pragmatic modular monolith: a Go API (with in-process background analysis runs), a Next.js frontend, and PostgreSQL. The anomaly engine is a pure, deterministic Go package. Generative AI is an optional explanation layer that never decides anomaly type, severity, confidence, or evidence.

```text
Browser → Next.js → /api/v1 → Go API → PostgreSQL
                                  └─ optional → Ollama (explanations only)
```

See [architecture](docs/architecture/architecture.md) and the [ADRs](docs/adr/).

## Approved Stack

| Area | Technology |
| --- | --- |
| Backend | Go, chi, pgx, sqlc, goose, slog |
| Database | PostgreSQL |
| Frontend | Next.js, React, TypeScript, Tailwind CSS, shadcn/ui, TanStack Query, Apache ECharts |
| Testing | Go testing, testify, testcontainers-go, Vitest, Testing Library, Playwright |
| Delivery | Docker, Docker Compose, GitHub Actions, OpenAPI 3 |
| Observability | Structured logs, health/readiness, Prometheus-compatible metrics |
| Generative AI (optional) | Ollama behind an explanation-provider boundary |

Dependency versions are pinned when each dependency is introduced.

## AI Philosophy

**Deterministic detection + optional generative explanation.**

Detection, classification (`REAL_ANOMALY`, `EXPLAINABLE_ANOMALY`, `FALSE_POSITIVE`, `DATA_QUALITY`), severity, confidence, priority, and evidence are computed by robust, reproducible statistics: hour-aware per-meter baselines (median/MAD, robust Z-score), persistence, multivariate and physical-consistency checks, and event correlation. An LLM, when configured, only rewrites already-computed evidence into operator-friendly language, and the product works fully without it. See [anomaly analysis](docs/ai/anomaly-analysis.md) and [explainability](docs/ai/explainability.md).

## Dataset

- `readings.csv`: 12 meters, 14 days, 4,032 hourly readings of consumption (kWh), voltage (V), current (A), and power factor.
- `events.csv`: known operational and data events.
- Location: `data/input/` (see [data/input/README.md](data/input/README.md)). The files are copied unchanged and are not generated.
- The evaluator's ground-truth file is intentionally not part of this repository and is never used.

## Repository Structure

```text
.agents/            shared coding-agent context and skills
.github/            Copilot instructions (CI workflows planned)
data/input/         source CSVs
database/           migrations (goose); queries (sqlc) from Phase 03
docs/               product, architecture, ADRs, AI, design, phases, quality, testing, performance
src/backend/        Go module: API, migrate and seed commands
src/frontend/       Next.js app — planned
tests/e2e/          Playwright journeys — planned
compose.yaml        local PostgreSQL runtime
.env.example        local-only placeholder configuration
AGENTS.md           engineering instruction router for contributors and agents
```

## Roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 00 | Repository, agent, and engineering governance foundation | Complete |
| 01 | Runtime foundation, PostgreSQL, verified idempotent dataset ingestion | Complete |
| 02 | Deterministic, evidence-producing anomaly engine | Complete (audited; non-blocking limitations documented) |
| 03 | Versioned, documented API with persisted analysis runs | Planned |
| 04 | Responsive product UI in the canonical visual language | Planned |
| 05 | Grounded explanation and investigation experience | Planned |
| 06 | Complete regression, observability, reproducible delivery, CI, demo | Planned |

Each phase starts only with explicit authorization, passes an entry gate (the previously accepted baseline must still pass) and an exit gate (objective evidence). See [roadmap](docs/phases/roadmap.md).

## Quality Approach

Testing is continuous: every phase ships its own unit, integration (real PostgreSQL), and — once the UI exists — Playwright functional tests, and reruns the earlier regression suite. The four challenge scenarios are automated acceptance tests without meter-specific production logic. See [testing strategy](docs/testing/testing-strategy.md) and [Definition of Done](docs/quality/definition-of-done.md).

## Design

The product follows [the design contract](docs/design/design-system.md), which adapts the canonical visual reference [`docs/design/reference/design-system-v3.html`](docs/design/reference/design-system-v3.html) (navy primary, Plus Jakarta Sans) to Energy Management semantics. The reference is a visual sample, not a source of requirements.

## Local Development

Prerequisites: Go 1.27.x and Docker with Compose v2 (see [environment readiness](docs/quality/environment-readiness.md)). Run every command from the repository root; `go -C src/backend` runs Go inside the backend module. These commands were verified in PowerShell and Bash. The frontend and a single one-command demo come in later phases (OD-19).

1. Start PostgreSQL 18 (localhost:5432) and wait until it is healthy:

   ```sh
   docker compose up -d --wait
   ```

2. Point the commands at the database (local-only credentials from `compose.yaml`):

   ```powershell
   $env:DATABASE_URL = "postgres://bia:bia_local_dev@localhost:5432/bia_energy?sslmode=disable"   # PowerShell
   ```

   ```sh
   export DATABASE_URL="postgres://bia:bia_local_dev@localhost:5432/bia_energy?sslmode=disable"    # Bash/zsh
   ```

3. Create the schema, then import `data/input/readings.csv` and `events.csv`. Re-running either command is safe:

   ```sh
   go -C src/backend run ./cmd/migrate up
   go -C src/backend run ./cmd/seed
   ```

   `migrate status` lists migrations and `migrate down` rolls back the latest one. The import validates both files before writing, loads them in one transaction and reports inserted, updated and unchanged rows.

4. Start the API (default `HTTP_ADDR=:8080`; set `LOG_LEVEL=debug` for more output) and check it:

   ```sh
   go -C src/backend run ./cmd/api
   curl http://localhost:8080/healthz   # {"status":"ok"}: the process is alive
   curl http://localhost:8080/readyz    # {"status":"ready",...}: PostgreSQL reachable, otherwise 503
   ```

5. Run the tests:

   ```sh
   go -C src/backend test ./...                     # unit tests and the analytics acceptance suite, no Docker needed
   go -C src/backend test -tags=integration ./...   # plus integration, acceptance and black-box tests (Docker required)
   ```

   The black-box test compiles the three commands and runs them as separate processes against a disposable PostgreSQL container.

6. Stop PostgreSQL. `docker compose stop` keeps the data; `docker compose down -v` deletes this project's database volume:

   ```sh
   docker compose stop
   ```

Schema and ingestion details: [data model](docs/architecture/data-model.md).

## Contributing

Read [AGENTS.md](AGENTS.md) first. Quality gate: [Definition of Done](docs/quality/definition-of-done.md). Testing approach: [testing strategy](docs/testing/testing-strategy.md).

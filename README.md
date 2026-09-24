# Bia Energy Management Platform

An MVP for electrical meter management that turns energy data into operational decisions: it detects anomalies, explains them with evidence, prioritizes what to investigate first, and recommends an action.

> **Current status:** Phase 00 (repository and engineering governance foundation) is complete. **The application is not implemented yet.** No commands in this repository run a product today.

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
database/           migrations (goose) and queries (sqlc) — planned
docs/               product, architecture, ADRs, AI, design, phases, quality, testing, performance
src/backend/        Go API — planned
src/frontend/       Next.js app — planned
tests/integration/  real-PostgreSQL and cross-component tests — planned
tests/e2e/          Playwright journeys — planned
AGENTS.md           engineering instruction router for contributors and agents
```

## Roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 00 | Repository, agent, and engineering governance foundation | Complete |
| 01 | Runtime foundation, PostgreSQL, verified idempotent dataset ingestion | Planned |
| 02 | Deterministic, evidence-producing anomaly engine | Planned |
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

*To be completed during bootstrap (Phases 01, 04, and 06).* The goal is a single documented mechanism that starts dependencies, migrates the database, imports the provided data, and starts backend and frontend. Prerequisites, environment variables, and commands will be documented here only once they exist and have been verified.

Toolchain baseline (validated on the development machine, see [environment readiness](docs/quality/environment-readiness.md)): Go 1.27.x, Node.js 24.x LTS, pnpm 12.x, Docker Desktop with Compose, PostgreSQL 18.x via `postgres:18.6-alpine`, and optionally Ollama.

## Contributing

Read [AGENTS.md](AGENTS.md) first. Quality gate: [Definition of Done](docs/quality/definition-of-done.md). Testing approach: [testing strategy](docs/testing/testing-strategy.md).

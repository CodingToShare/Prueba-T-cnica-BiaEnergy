# Bia Energy Management Platform — Engineering Instruction Router

This file is the canonical instruction router for every contributor and repository-aware coding agent. It routes to canonical documents instead of repeating them. `CLAUDE.md` and `.github/copilot-instructions.md` delegate here; if they ever disagree with this file, this file wins and the disagreement is a defect to fix.

For a fast bootstrap, read `.agents/context/project-context.md` first.

## 1. Source Of Truth Hierarchy

Authority is category-specific. Higher entries win within their category:

1. The original technical challenge, as consolidated in `docs/product/` (requirements and business rules).
2. Accepted ADRs in `docs/adr/` (architecture decisions).
3. `docs/architecture/architecture.md` and `docs/ai/` (how the approved architecture and analytics behave).
4. `docs/design/design-system.md` (product visual contract), with `docs/design/reference/design-system-v3.html` as the canonical visual reference.
5. This file (engineering and delivery rules).
6. `.agents/skills/*/SKILL.md` (area playbooks).
7. Existing implementation.

An assumption, open decision, or implementation detail never overrides a requirement or an accepted decision. Do not silently reverse an accepted ADR; supersede it with a new ADR.

## 2. Mandatory Workflow

Every agent follows this sequence for implementation work:

```text
Identify the explicitly authorized phase
  → Read applicable context (this file, project context, phase doc, requirements, ADRs, skills, DoD)
  → Run the Phase Entry Gate
  → Report baseline problems, if any ("Blocked by baseline regression")
  → Implement only the authorized scope
  → Add/update tests together with the implementation
  → Run focused validation
  → Run regression validation
  → Review the actual diff
  → Perform manual and design acceptance where relevant
  → Update documentation + traceability
  → Evaluate the Definition of Done
  → Mark Complete only with evidence
  → STOP (never begin the next phase automatically)
```

Before editing code, identify and state: the active authorized phase (`docs/phases/roadmap.md`); the product requirement(s) (`docs/product/`); the applicable ADR(s); the applicable skill(s) (table below); the expected tests (`docs/testing/testing-strategy.md`); and the Definition of Done items that apply (`docs/quality/definition-of-done.md`). If any cannot be identified, stop and surface the gap instead of guessing.

**Phase Entry Gate** (full procedure in the `phase-governance` skill): confirm the previous phase is Complete; inspect `git status` and preserve unrelated changes; run the applicable baseline validation (roadmap); confirm accepted builds/tests still pass; validate previous evidence and known limitations; confirm the work belongs to the authorized phase. A broken baseline is reported, never silently bypassed; regressions of accepted work are repaired before new behavior is added.

## 3. Task Routing

| Task | Read | Skill |
| --- | --- | --- |
| Product behavior, scope, priorities | `docs/product/*` | `documentation` |
| Module boundaries, dependencies, new libraries | `docs/architecture/architecture.md`, `docs/adr/*` | `architecture` |
| Go API, handlers, services, orchestration | ADR-002, ADR-007, architecture | `go-backend` |
| Schema, migrations, queries, indexes | ADR-003, `docs/performance/data-query-strategy.md` | `postgresql` |
| CSV loading, validation, seeding | `data/input/README.md`, architecture | `data-ingestion` |
| Baselines, detection, classification, severity, confidence | ADR-004, `docs/ai/anomaly-analysis.md` | `analytics-engine` |
| Explanations, recommendations, LLM provider | ADR-006, `docs/ai/explainability.md` | `ai-explainability` |
| Next.js pages, data fetching, charts | ADR-005, `docs/design/design-system.md`, `docs/design/reference/design-system-v3.html` | `nextjs-frontend`, `ui-design-system` |
| Logging, health, metrics | architecture (observability) | `observability` |
| Tests at any level | `docs/testing/testing-strategy.md` | `testing` |
| Starting, executing or closing a phase | `docs/phases/*` | `phase-governance` |
| Reviewing a diff or declaring completion | DoD | `code-review` |

## 4. Architecture Principles

Pragmatic modular monolith (ADR-001): one Go API process, one Next.js frontend, one PostgreSQL database. Product-first, API-first, explicit and testable code, KISS, YAGNI, SOLID with judgment. Scale by evolution, not speculation.

Do not introduce any item listed in `docs/product/out-of-scope.md` (microservices, brokers, Redis, Kubernetes, vector databases, RAG, agent frameworks, a Python ML service, TimescaleDB, full CQRS, event sourcing, generic repositories, complex auth) without a new accepted ADR that demonstrates a real requirement.

## 5. Backend Standards (Go)

- Idiomatic Go; packages by domain capability under `src/backend/internal/` (ADR-002). Do not port layered project structures from other ecosystems.
- HTTP handlers are thin: decode, validate input shape, call a service, encode. No business or analytics rules in handlers.
- Interfaces only at meaningful boundaries: persistence where it adds testability, `ExplanationProvider`, `Clock` when time must be deterministic. Define interfaces where they are consumed.
- Propagate `context.Context` through every I/O call. Wrap errors with context; map them to HTTP centrally with a consistent error body.
- Structured logging with `slog`; never log secrets or full request bodies.
- Source observation times are timezone-naive `timestamp` values holding the source wall clock (ADR-008); system-generated instants use `timestamptz`. Never invent or convert a zone.

## 6. Frontend Standards (Next.js)

- Next.js + React + TypeScript (strict) + Tailwind CSS + shadcn/ui + TanStack Query + Apache ECharts (ADR-005).
- Feature-oriented organization. TanStack Query owns server state; no global state library unless real complexity is demonstrated.
- Every data view handles loading, empty, error and retry states. Accessibility and responsive behavior are required, not polish.
- Before any UI work, read `docs/design/design-system.md` (implementation contract) and open `docs/design/reference/design-system-v3.html` (canonical visual reference: navy primary, Plus Jakarta Sans). The reference shows the visual language applied to this product with illustrative values; it is never a source of requirements or data, and it changes only through an explicit, recorded design decision.
- Use semantic tokens; no scattered hex values; no default framework theme or second design system. Severity, anomaly type, and destructive actions are separate visual concepts.
- Validate significant screens at ~1440/768/390 px against the reference before completing UI work.

## 7. Data And Analytics Standards

- PostgreSQL is the source of truth (ADR-003). Filtering, sorting, and aggregation that belong to the database happen in SQL.
- The anomaly engine is deterministic, reproducible, and unit-testable without a database or network (ADR-004).
- **Never** branch on specific meter IDs, timestamps, or other dataset-specific literals to reach an expected answer. Acceptance scenarios (M-104, M-106, M-109, M-112) may appear only in tests and documentation.
- **Never** create, search for, read, or depend on `expected_results.csv`. It is reserved for the evaluator.
- Never modify source CSVs in `data/input/`.
- Thresholds and weights are calibration decisions: they live in one named configuration location, are documented in `docs/ai/anomaly-analysis.md` with rationale, and are never scattered as magic numbers.
- Keep source/device status (`source_status`) distinct from analytically computed status (`computed_status`).

## 8. AI And Explainability Standards

- The LLM is **not** the anomaly detector (ADR-006). It must never decide anomaly existence, type, severity, numeric confidence, evidence, or baselines.
- The product must work fully with no LLM configured; the deterministic explanation provider is the default and the fallback.
- Generated text must be grounded exclusively in already-computed structured evidence. No invented metrics, events, or causes.
- Confidence is derived from signals (detection strength, persistence, multivariate confirmation, event-correlation certainty, data reliability), never a hard-coded constant.

## 9. Testing Standards

Follow `docs/testing/testing-strategy.md`. **Testing is continuous, not deferred to Phase 06**: every phase adds unit, integration, and (once a frontend exists) functional tests for its own behavior and reruns the previously applicable regression suite. Sequence: Implement → Validate → Review → Document → Complete. Test behavior and risk; avoid coverage theater and excessive mocking. The main suite never depends on a live LLM. Go unit tests live beside the package (`*_test.go`); Go integration tests (real PostgreSQL via testcontainers) also live beside their package, behind the `integration` build tag (TD-20); frontend unit tests live beside the code; `tests/e2e/` holds Playwright journeys. The four acceptance scenarios must be verified semantically once the engine exists.

## 10. Phase Governance

Gates (defined in `docs/phases/roadmap.md`): phase authorization, entry, scope, test, design, documentation, exit, next-phase.

- Execute only a phase the user has **explicitly authorized**. Authorization is never inferred; finishing a phase never authorizes the next one.
- Run the Phase Entry Gate before new work and the Phase Exit Gate before claiming completion.
- Stay within the phase scope and non-goals; do not pull later work forward silently.
- A phase is Complete only with recorded objective evidence and no known blocking failure, never because files exist.
- Update the phase document and `docs/product/traceability-matrix.md` in the same change as the work.

## 11. Documentation And ADRs

- Keep requirement, business rule, technical decision, assumption, open decision, and out-of-scope items in their categories. Never promote an assumption to a requirement silently.
- Record significant decisions (architecture style, major library, persistence strategy, AI responsibility, execution model, public API strategy) as an ADR using the existing template.
- Document facts in one canonical place and link to it. Do not describe planned work as implemented.
- Product documentation (`docs/`, `README.md`) is tool-neutral: decisions belong to the project, not to a specific assistant. Do not attribute work to coding agents in tracked product artifacts or commit messages.

## 12. Security And Secrets

- No secrets, credentials, tokens, or personal data in the repository. Configuration comes from environment variables; commit only `.env.example` with placeholders.
- Parameterized SQL only: sqlc-generated queries for product data access (from Phase 03); explicit parameterized pgx statements for bulk ingestion (TD-18). Never build SQL from strings. Validate all external input at the boundary.
- Do not expose stack traces or internal errors to clients.
- Authentication stays intentionally simple for the challenge; do not build OAuth/OIDC/RBAC.

## 13. Delivery

Docker Compose (`compose.yaml`, PostgreSQL since Phase 01) is the local runtime; GitHub Actions is the intended CI (Phase 06). By Phase 06 an evaluator must be able to start dependencies, migrate, import the provided data, and start backend and frontend without manually repairing state (FR-DEL-002; entry point OD-19). Never document a command before it exists and works. Pin dependency versions when they are first introduced, not before. Prefer Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `test:`). Commits are authored by the repository owner only and never carry `Co-Authored-By` or other attribution to coding assistants (details: `phase-governance` skill, "Commits").

## 14. Agent Configuration Files

`AGENTS.md`, `CLAUDE.md`, `.github/copilot-instructions.md`, and `.agents/` are **tracked, shared repository governance** so every agent sees the same rules. Per-user tool state (local settings, caches, sessions) is ignored in `.gitignore`. `docs/ai/` is **product** documentation about the platform's analytics and explainability; it is not agent configuration.

## 15. Completion Requirements

A change is complete only when: the affected projects build; relevant tests pass (report exact commands and results); architecture boundaries hold; error behavior is considered; no secrets, dead code, or speculative abstractions were added; documentation and traceability are consistent; the actual diff has been reviewed with the `code-review` skill; and no completion is claimed without evidence.

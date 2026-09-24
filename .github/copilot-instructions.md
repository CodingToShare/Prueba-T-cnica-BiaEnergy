# GitHub Copilot Repository Instructions

The canonical engineering instruction router is `AGENTS.md` at the repository root. Before generating or changing code, follow its routing to the active phase, product requirements, ADRs, design system, and the relevant playbooks in `.agents/skills/`. A concise bootstrap is in `.agents/context/project-context.md`.

Key constraints (authoritative text lives in `AGENTS.md`):

- Stack: Go backend (chi, pgx, sqlc, goose, slog), PostgreSQL, Next.js + TypeScript + Tailwind + shadcn/ui + TanStack Query + ECharts. Do not introduce other frameworks or infrastructure without an accepted ADR.
- Work only within the phase the user explicitly authorized, after the Phase Entry Gate confirms a green baseline.
- Add tests together with the implementation; run focused and regression tests before claiming completion.
- UI follows `docs/design/design-system.md` and the canonical visual reference `docs/design/reference/design-system-v3.html`.
- Anomaly detection is deterministic and generalized: no meter-specific branches, no use of `expected_results.csv`. Generative AI only rewords already-computed evidence and is optional.
- Keep handlers thin, SQL parameterized, secrets out of the repository, and tests focused on behavior.
- Apply `docs/quality/definition-of-done.md` and update phase evidence for authorized phase work.

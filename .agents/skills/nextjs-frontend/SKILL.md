---
name: nextjs-frontend
description: Build or modify the Next.js + TypeScript frontend under src/frontend, including routes, data fetching with TanStack Query, and ECharts visualizations.
---

# Next.js Frontend

## Use When

Any change under `src/frontend`.

## Responsibilities

Deliver the product flow in ADR-005 with feature-oriented code: `auth`, `dashboard`, `meters`, `anomalies`, `analysis`, plus a small shared UI layer.

## Required Rules

- Read ADR-005, `docs/design/design-system.md`, `docs/design/reference/design-system-v3.html`, and the `ui-design-system` skill first.
- TypeScript strict; typed API client generated from or checked against the OpenAPI document.
- TanStack Query owns server state: query keys per resource, polling for analysis progress, sensible retries, invalidation after a completed run.
- Client-side state stays local unless proven otherwise; no global store by default.
- Load ECharts only where used; theme it from tokens.
- Every data view handles loading, empty, error (with retry), and not-analyzed states.
- Display values as computed by the API; never recompute classifications or severities.
- Configuration via environment variables (`NEXT_PUBLIC_*` only for non-secret values).

## Prohibited

Analytics logic in the browser; `any` as an escape hatch; duplicating server data in global stores; secrets in client bundles; a second UI library.

## Completion Checklist

- Build, typecheck, lint, and component tests pass.
- States, accessibility, and responsive behavior verified at ~1440/768/390 px against the HTML reference.
- Playwright journeys added or updated in the same phase as the flow change.

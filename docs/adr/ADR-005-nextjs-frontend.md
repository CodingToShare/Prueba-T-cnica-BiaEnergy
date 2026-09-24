# ADR-005: Next.js + React + TypeScript Frontend

- Status: Accepted
- Date: 2026-09-24

## Context

Frontend/UX carries the largest single share of the evaluation together with backend and data. The product must feel like an Energy Management SaaS: consistent navigation, dashboards, filters, time-series charts with event markers, accessible and responsive states.

## Decision

- Next.js with React and strict TypeScript, in `src/frontend/`.
- Tailwind CSS with semantic design tokens and shadcn/ui components (copied into the repository, owned and themed by us).
- TanStack Query owns all server state (fetching, caching, polling of analysis progress, retries). No global state library unless real client-state complexity appears.
- Apache ECharts for time-series and distribution charts, themed from the same design tokens.
- Feature-oriented organization (dashboard, meters, anomalies, analysis, auth) with shared UI primitives only when reused.
- The frontend consumes only the versioned HTTP API; it contains no analytics rules.
- Unit/component tests with Vitest and Testing Library; journeys with Playwright.

## Consequences

- Fast delivery of a polished UI with accessible primitives.
- Next.js adds server/client component decisions; data fetching stays in client components via TanStack Query unless a clear benefit exists.
- ECharts is heavy; load charts only on pages that use them.

## Alternatives Considered

- **Vite + React SPA**: lighter, but loses built-in routing and layouts that speed up a multi-page product.
- **Recharts / Chart.js**: simpler, but weaker for dense time series with zoom and event markers.
- **Redux/Zustand for server data**: duplicates what TanStack Query already does.

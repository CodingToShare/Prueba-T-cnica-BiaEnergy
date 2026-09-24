# Phase 04 — Frontend Product Experience

- Status: **Planned / Not Started**
- Authorization: requires explicit user authorization.

## Objective

A responsive, SaaS-quality product UI in the canonical visual language, covering `Login → Dashboard → Meters → Meter Detail → Run AI Analysis → AI Anomalies → Investigation → Recommended Action` (the investigation experience is deepened in Phase 05).

## Prerequisites

- Phase 03 Complete, or an explicitly authorized parallelization after API contracts are stable.
- Phase Entry Gate: backend build/tests, API integration tests, OpenAPI/contract checks.

## Scope

- Bootstrap Next.js (TypeScript strict), Tailwind, shadcn/ui, TanStack Query, ECharts; pin versions; tokens from `docs/design/design-system.md`; resolve OD-13 (UI language).
- **Login:** the simple challenge entry experience (unless requirements change).
- **Dashboard:** meter count, total consumption, anomalies, high-priority anomalies, aggregate confidence, last analysis state/time.
- **Meters:** search by `meter_id`, status filtering (all/normal/alert/critical), sorting (consumption, variation, severity), consumption, variation, anomaly/severity indication.
- **Meter detail:** current/period consumption, baseline, variation, state, historical time series of consumption, voltage, current, power factor, with event and anomaly markers.
- **Run AI Analysis:** staged progress and completion summary.
- **AI anomalies:** meter, type, severity, confidence, recommended action, priority order.
- **Investigation (baseline version):** what was found, changed variables, baseline comparison, related events, severity, confidence, structured evidence, recommended action.
- Loading, empty, error-with-retry, disabled, and not-analyzed states; responsive layout with explicit mobile navigation.
- No additional product modules beyond these.

## Non-Goals

Analytics logic in the browser, LLM integration, CI.

## Expected Validation

- Frontend build, lint, TypeScript typecheck.
- Component/unit tests: states (loading, empty, error), filters/sort/search, formatting, accessibility basics.
- Playwright critical flows against the real API (Login → Dashboard → Run AI Analysis → anomalies → M-109) plus meter search/filter and detail navigation.
- Design review at ~1440/768/390 px and side-by-side comparison with `docs/design/reference/design-system-v3.html`.
- Regression: complete backend suite.

## Evidence Required

Entry Gate result; commands and counts; screenshots or review notes per viewport; Exit Gate checks; traceability rows FR-DASH-*, FR-MTR-*, FR-ANL-001/003, FR-ANOM-001, FR-INV-001, FR-AUTH-001, FR-UX-001 updated.

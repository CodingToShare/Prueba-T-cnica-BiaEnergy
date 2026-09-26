# Bia Energy — Frontend

Next.js (App Router) product UI for the Bia Energy Management Platform. Setup, commands and tests are in the [repository README](../../README.md#manual-development); the visual contract is [docs/design/design-system.md](../../docs/design/design-system.md).

```text
app/          routes: /login and the authenticated (app) group — dashboard, meters, anomalies
components/   shared UI: badges, layout (panels, KPI cards), states, segmented control, ui/ (shadcn/ui)
features/     one folder per product area: auth, shell, dashboard, analysis, meters, anomalies
lib/api/      fetch client, one function per API operation, types generated from docs/api/openapi.yaml
lib/format/   display formatting (numbers, source vs system times, labels)
lib/query/    TanStack Query client, keys and the global 401 handling
test/         Vitest setup and helpers; unit/component tests live beside the code (*.test.ts[x])
e2e/          Playwright specs and the real-stack runner (pnpm test:e2e)
```

The browser calls only same-origin `/api/v1/*`; Next.js forwards it to the server-only `BACKEND_URL`. After changing `docs/api/openapi.yaml`, run `pnpm generate:api`.

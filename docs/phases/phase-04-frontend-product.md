# Phase 04 — Frontend Product Experience

- Status: **Complete** (independent audit; uncommitted)
- Authorization: explicitly authorized by the user, together with the Phase 03 checkpoint commit. Phase 05 is not authorized and was not started.
- Date: 2026-09-24/25
- Phase 03 checkpoint: local commit `2908b94` "feat: add persistent analysis API and orchestration" (author Santiago Forero, no co-author trailer; not pushed; no remote).

## Objective

A responsive product UI in the canonical visual language covering `Login → Dashboard → Meters → Meter Detail → Run AI Analysis → AI Anomalies → Investigation → Recommended Action`. The UI consumes the Phase 03 API and never re-implements analytics. The generated explanation layer is Phase 05.

## Phase 03 Checkpoint

Pre-commit regression on the Phase 03 tree: unit 154 top-level tests + 89 subtests; integration/black-box 199 + 124 (including `TestBlackBox_AuthenticatedAnalysisGoldenPath`), 0 failed and 0 skipped; `go mod verify`, `go vet`, `git diff --check` and sqlc `diff` clean. Committed as `2908b94`; the tree was clean afterwards, apart from the ignored `tmp/` and `src/backend/tmp/`.

## Phase Entry Gate

| Check | Result |
| --- | --- |
| Previous phase | Phase 03 Complete and committed (`2908b94`) |
| Working tree | Clean before Phase 04 work |
| Backend baseline | The regression above, run on the same tree immediately before the commit |
| API contract | `docs/api/openapi.yaml` read and turned into generated types; Phase 03 evidence reviewed (dataset outcome M-109, M-112, M-104, M-106) |

## Contract Documentation Fix

`docs/api/openapi.yaml` had two flow-mapping descriptions with an unquoted comma: `variation_pct` in `AnomalySummary` and `high_priority` in `DashboardSummary`. YAML cut each description at the comma and produced a stray key, which showed up in the generated TypeScript types. Both descriptions are now quoted; the wording is unchanged. This is a documentation fix only: backend code and behavior are unchanged, and the Go OpenAPI tests pass.

## Stack (exact versions)

Node 24.21.0 · pnpm 12.6.0 (`packageManager`) · Next.js 16.3.6 (App Router, Turbopack) · React 19.2.8 · TypeScript 5.9.3 (strict) · Tailwind CSS 4.3.3 · shadcn 4.21.0 CLI, components copied into `components/ui` (Radix base via `radix-ui` 1.6.7) · TanStack Query 5.103.2 · ECharts 6.1.0 · lucide-react 1.48.0 · Vitest 5.0.1 + jsdom 30.1.1 + Testing Library (react 16.3.3, dom 10.4.2, user-event 14.6.7, jest-dom 7.0.1) · Playwright 1.63.0 (Chromium 1243) · openapi-typescript 7.13.0 · ESLint 9.39.5 with `eslint-config-next` 16.3.6.

## Decisions

Recorded in `docs/product/assumptions-and-decisions.md`: TD-26 (frontend structure and pinning), TD-27 (same-origin rewrite), TD-28 (generated API types), TD-29 (test locations and the real-stack E2E runner; the `tests/e2e/` placeholder was removed), plus the Phase 04 resolutions: OD-13 (English UI), login UI and session handling (OD-12), time display, analysis polling and the chart baseline.

## Architecture

```text
app/                 routes (Server Components): / → redirect; /login; (app)/ group with a server layout guard
  (app)/dashboard, meters, meters/[meterId], anomalies, anomalies/[id], error.tsx; not-found.tsx
features/            client views per product area: auth, shell, dashboard, analysis, meters (+ chart), anomalies
components/          badges, layout (PageHeader, Panel, KpiCard, MeterBar), states, segmented, ui/ (shadcn)
lib/api              fetch client (ApiError), one function per operation, generated schema types
lib/format           numbers, source vs system time, labels/tones
lib/query            QueryClient policy, keys, providers (global 401)
```

- **Server vs client:** route files are Server Components. They resolve `params`/`searchParams`, set metadata and check only whether the session cookie is present (`cookies().has("bia_session")`, never parsed) so they can redirect anonymous visitors. Product views are Client Components that fetch through TanStack Query. The API stays the authority for every request.
- **API client:** same-origin `fetch` (`credentials: "same-origin"`, `cache: "no-store"`). The standard error body becomes `ApiError(status, code, message, requestId)`. Unexpected bodies, gateway errors and network failures turn into safe generic messages.
- **Same-origin rewrite:** `next.config.ts` rewrites `/api/v1/:path*` to `${BACKEND_URL}/api/v1/:path*`. `BACKEND_URL` is server-only (no `NEXT_PUBLIC_*` variable exists) and is resolved when `next dev` / `next build` start.
- **TanStack Query:** staleTime 30 s, no refetch on focus, no retry for 4xx. Every 401 goes through the QueryCache/MutationCache `onError` to one redirect handler; the login mutation opts out with `meta.handlesUnauthorized`. List views use `keepPreviousData`.
- **Session UX:** login posts once; the backend sets the HttpOnly cookie; the session is read with `GET /api/v1/auth/session`. JavaScript never reads or writes the cookie, and nothing goes to local or session storage. Logout calls `POST /auth/logout` and replaces the route with `/login`; the cache is cleared when the login page shows. Any 401 redirects once to `/login?expired=1`, and never while on `/login`, so there is no redirect loop.

## Routes

| Route | Purpose | Authentication |
| --- | --- | --- |
| `/` | Redirects to `/dashboard` or `/login` | Cookie presence |
| `/login` | Sign in; "session ended" notice with `?expired=1` | Public |
| `/dashboard` | KPIs, Run AI Analysis, progress/completion, investigation queue, fleet health | Required |
| `/meters` | Search, status filter, sort, pagination | Required |
| `/meters/[meterId]` | Meter KPIs, current finding, reading history chart | Required |
| `/anomalies` | Findings in priority order with severity/type/meter filters | Required |
| `/anomalies/[id]` | Investigation of one finding (numeric id validated) | Required |
| any other path | Not-found page | — |

## Product Behavior

- **Dashboard.** *Before analysis:* real meter count, readings and total consumption; anomalies, high priority, confidence and latest analysis show "—" with "Not analyzed yet"; the queue shows "No analysis yet"; fleet health shows every meter as "Not analyzed". *During:* the button is disabled ("Analyzing…"); a live status panel shows the real stage, "Step X of 5" and a progress bar. Previous results stay visible. *After:* the completion alert shows the counts from the run ("4 anomalies detected · 2 require high-priority attention") with a link to the anomalies; KPIs, queue and fleet health refresh once. *Failure:* the error message from the run, a note that previous results remain, and "Retry analysis".
- **Meters.** Debounced literal search; All/OK/Alert/Critical segmented filter (`aria-pressed`); sortable Meter, Consumption, Variation (by magnitude, server-side) and Severity headers (`aria-sort`); pagination of 10 rows. `null` status shows "Not analyzed" and missing values show "—", never 0%. Stacked labeled rows at ≤ 640 px.
- **Meter detail.** KPIs: computed status, period consumption, readings, variation. Current-finding panel (badges, backend reason, episode, action, "Open investigation"). Reading history with a Consumption/Voltage/Current/Power factor selector. Overlays come only from stored evidence (episode region, events at their exact reading times, engine baseline at flagged hours), with a legend, an accessible summary and a data-table alternative.
- **AI anomalies.** Backend priority order with priority number, meter, type, severity, confidence, recommended action and episode start. Severity, type and meter-ID filters are sent to the API; an invalid meter ID is flagged and not sent. False positives stay visible.
- **Investigation.** Answers *what happened* (backend reason), *why it matters* (type meaning), *what supports it* (stored deviation, duration, supporting variables) and *what to do* (action card). A type-specific callout is built from the evidence: a REAL anomaly with context-only events says none explains the change; EXPLAINABLE names the explaining event and its offset; FALSE_POSITIVE names the explaining event and the observed recovery; DATA_QUALITY lists the inconsistent variables while consumption stayed within its bound. Event roles are badges; UNKNOWN/CONTEXT reads "Context only — does not explain it". The page also has a changed-variables table, the chart, confidence components (labelled as not a failure probability) and collapsed technical evidence (rule, strength, signals table). API text is rendered as text. There is no meter-specific logic: every sentence comes from type, role and evidence fields.
- **Times.** Source timestamps are formatted from their string parts ("Sep 12, 2026, 14:00"), unaffected by the browser time zone. The E2E runs in `America/Bogota` and asserts the source hour. System instants (latest analysis) use the browser zone.

## Test Evidence

Implementation checkpoint before the independent audit. The expanded final evidence is recorded under **Final Audit Evidence** below.

| Gate | Command | Result |
| --- | --- | --- |
| Lint | `pnpm lint` | Clean |
| Typecheck | `pnpm typecheck` (`next typegen && tsc --noEmit`) | Clean |
| Unit + component | `pnpm test` | 13 files, **64 tests passed**, 0 failed, 0 skipped |
| Build | `pnpm build` | Success; 8 routes |
| E2E (real stack) | `pnpm test:e2e` | **4 passed** (desktop ×2, mobile, tablet), ~20 s of tests, ~40 s end to end including build |
| Backend unit | `go -C src/backend test -json -count=1 ./...` | 154 top-level + 89 subtests passed, 0 failed/skipped |
| Backend integration | `go -C src/backend test -json -tags=integration -count=1 ./...` | 199 top-level + 124 subtests passed, 0 failed/skipped (testcontainers PostgreSQL 18.6) |

Vitest files: `time.test.ts` 6 (source times unchanged under `TZ=Pacific/Kiritimati`, strict parsing, periods, system zones), `numbers.test.ts` 5, `labels.test.ts` 4, `client.test.ts` 7 (error body, safe fallbacks, same-origin credentials, 204, network error), `state.test.ts` 3, `evidence.test.ts` 5, `option.test.ts` 5 (categories, metric, overlays only at exact times, tooltip escaping, text summary), `login-form.test.tsx` 4, `dashboard-view.test.tsx` 7 (honest before, after, error + retry, first poll already COMPLETED, progress → FAILED stops polling + retry, active-run recovery, start error), `meters-view.test.tsx` 5, `anomalies-view.test.tsx` 3, `investigation-view.test.tsx` 7 (all four types, context-only event, HTML-like text rendered as text, not found), `providers.test.tsx` 3 (401 → login once, no redirect on /login, logout without refetch + cache cleared).

Component tests replace `fetch` (a route table) and `next/navigation`; investigation tests replace the separately tested chart, and the chart lifecycle test substitutes ECharts and ResizeObserver. TanStack Query remains real; the component helper disables query retries for speed, while provider and browser tests retain the production retry policy. Mutation checks from implementation: making polling unconditional fails the two polling tests; clearing the cache before navigating on logout fails the logout test.

**Playwright (network mocked: NO).** `e2e/run-e2e.mjs` starts a disposable `postgres:18.6-alpine` container with a random password on a free port, builds and runs `migrate up`, `seed` (the challenge CSVs) and the API with random test-only credentials (`SESSION_COOKIE_SECURE=false`), readiness-polled. It builds and starts the production frontend with `BACKEND_URL` pointing at that API, runs Playwright, and removes processes (process tree) and the container, also on failure. No fixed sleeps: readiness polling and web-first assertions only. After each run, `docker ps -a --filter name=bia-e2e` was empty.

- `desktop` 1440×900: invalid login (keyboard submit, alert, password cleared, still on `/login`). The golden path: anonymous `/dashboard` → `/login`; sign in; skip link is the first Tab stop; before: 12 meters, 155,250.85 kWh, "—"; Run AI Analysis → "4 anomalies detected · 2 require high-priority attention"; 4 / 2 / 83%; queue M-109, M-112, M-104, M-106; AI Anomalies table (4 rows, M-109 "Investigate meter and installation"); M-109 investigation (episode "Sep 12, 2026, 14:00" in a UTC−5 browser, context-only UNKNOWN event, action, chart, technical evidence); Meters "Showing 1–10 of 12", search M-109 → Critical; meter detail with 336 readings and consumption/voltage charts; sign out → `/login`; `/meters/M-109` → `/login`; `GET /api/v1/auth/session` → 401.
- `mobile` 390×844 and `tablet` 768×1024 (depend on `desktop`): menu sheet navigation, dashboard queue, meters, anomalies, investigations of M-112 (data quality), M-106 (false positive), M-104 (explainable) and M-109 (real) with type callouts, meter detail chart, sign out from the sheet. No horizontal overflow on every screen checked.
- Every test fails on console errors/warnings, page errors, failed requests and HTTP ≥ 400, apart from the expected 401 of the invalid login.

## Real Dataset UX Result

| Priority | Meter | Type | Severity | Confidence |
| --- | --- | --- | --- | --- |
| 1 | M-109 | Real anomaly | High | 93% |
| 2 | M-112 | Data quality | High | 92% |
| 3 | M-104 | Explainable anomaly | Medium | 73% |
| 4 | M-106 | False positive | Low | 76% |

Dashboard after analysis: 12 meters, 4,032 readings, 155,250.85 kWh, 4 anomalies, 2 high priority, AI confidence 83% (mean of the four findings); fleet health: 1 Critical, 2 Alert, 9 OK.

## Visual And Responsive Review

Screenshots from the real-stack run (desktop: login, dashboard before/after, anomalies, M-109 investigation, meters, M-109 detail; mobile and tablet: dashboard, M-109 and M-112 investigations) were reviewed next to `docs/design/reference/design-system-v3.html`. They are regenerated by every `pnpm test:e2e` under the ignored `src/frontend/test-results/` and were not committed. Issues found and fixed during the review:

1. Chart: the "Anomaly episode" label collided with the event label and was clipped at the right edge; the region is now named by the legend only.
2. Investigation: the three story blocks left an empty grid cell and the action card stretched with blank space; the summary now takes two thirds of the row and the action card aligns to the top.
3. Meter detail: the computed status appeared twice (text and badge); it is now one badge with its source as context.
4. Pre-analysis fleet health said "from the latest completed analysis"; it now says the meters are not analyzed yet.
5. Mobile KPI: "155,250.85 kWh" broke inside the number and the unit; the KPI value scales down on phones (capped at the reference 1.7 rem) and the unit never splits.
6. Dashboard KPI breakpoint aligned to the contract (3 columns, 2 at ≤ 720 px); meter-detail KPIs use 4 columns only from 1024 px.
7. Placeholders naming dataset meters ("e.g. M-109") replaced by "Meter ID".

The full-page capture shows the sticky sidebar only at the top of the image; this is an artifact of full-page screenshots, and the sidebar stays in view in the browser.

## Accessibility Review

`lang="en"`; skip link (E2E: first Tab stop); landmarks (`nav` "Main navigation", `main`); one `h1` per page with ordered sections; labelled inputs with `aria-invalid` and described hints; `aria-pressed` segmented filters; `aria-sort` headers; `aria-current="page"` navigation; `role="status"`/`aria-live` progress and completion; `role="alert"` errors; progressbar with values; the Radix sheet manages focus trapping, Escape and `aria-expanded`. Badges carry text plus a distinct glyph (color is never the only cue). Charts have an accessible summary and a data table. Reference AA token values, visible focus outlines (white on navy), reduced-motion support and 44 px navigation targets. No automated axe audit was run; that is recorded as Phase 06 work.

## Console / Network, Security And Bundle

- Console/network: every E2E test asserts a clean console and network, and all passed. The review found and fixed a real defect: logout cleared the query cache while the shell was still mounted, so the session query refetched and got a 401 that could race the global handler into "session expired". The cache is now cleared on the login page, with a component test that fails on the old behavior.
- Secrets: the production client chunks (`.next/static`) contain none of `DATABASE_URL`, `DEMO_AUTH`, `SESSION_SIGNING`, `BACKEND_URL`, `postgres://`, loopback backend addresses, `localStorage`, `sessionStorage`, `document.cookie` or `NEXT_PUBLIC`. The only `dangerouslySetInnerHTML` hits are React DOM internals; the source has none. The ECharts tooltip escapes all text. `.env*` is ignored except `src/frontend/.env.example` (placeholders). The E2E credentials are random per run and exist only in process environments.
- Headers: `poweredByHeader: false`.
- Dataset IDs appear only in E2E assertions and docs; the OpenAPI examples in the generated types come from the contract.

## Performance Sanity

Client JavaScript: 1,410 KB raw / 465 KB gzip across all chunks. ECharts (core + line chart only) is one lazily loaded chunk of 184 KB gzip, absent from every route's initial manifest and fetched only by the chart. Readings: one request per meter (limit 1000, cached 5 min); lists are paginated server-side; search is debounced (250–300 ms); polling runs only while a run is active. On the real stack the full golden path (login → analysis → investigation → meter → logout) takes about 5 s.

## Code Review / Overengineering

The actual diff was reviewed. No analytics, severity, confidence, status or baseline computation exists in TypeScript; the frontend only formats and arranges stored values. Removed during review: an unused Tooltip primitive and provider, the scaffold favicon (replaced by a brand SVG icon), the scaffold README and an empty `public/`. Kept small: no state library beyond TanStack Query, no form library, no date library.

The backend is unchanged. The only non-frontend source change is the OpenAPI description quoting.

## Phase Exit Gate

Implementation checkpoint before the independent audit; the audit reran and expanded every applicable gate.

| Check | Result |
| --- | --- |
| Scope complete (login, dashboard, meters, detail, analysis, anomalies, investigation, states, responsive) | Yes |
| Tests with the implementation (unit, component, real-stack E2E) | Yes, all passing |
| Backend regression | Passing, unchanged counts |
| Lint / typecheck / build | Clean |
| Visual review at 1440 / 768 / 390 vs the reference | Done; issues fixed |
| Documentation and traceability | README, design-system §10, testing strategy, decisions, traceability, repo maps, this record |
| No meter-specific logic, no secrets, no raw JSON as the primary UX | Verified |
| Phase 04 commit | Not created, by instruction (left for independent audit) |

## Known Limitations And Deferred Work

- Phase 05: generated (optional LLM) explanation with grounding and fallback in the investigation; a richer investigation narrative beyond the deterministic story blocks.
- Phase 06: automated accessibility audit (axe); CI running lint/typecheck/Vitest/Playwright; one-command demo (OD-19); observability of the frontend; cross-browser runs (Firefox/WebKit); an hourly baseline series endpoint if a continuous baseline band is required in the chart; performance budgets.
- No server-side session revocation (Phase 03 limitation); the frontend route guard checks cookie presence only, and the API rejects invalid sessions.
- `BACKEND_URL` is fixed at build time for production builds (Next.js rewrites).

## Next Phase

None authorized. Phase 05 was not started and requires explicit user authorization.

## Independent Frontend / UX Audit — 2026-09-25

This audit inspected the uncommitted implementation against Phase 03 HEAD `2908b94`, ran the actual PostgreSQL → compiled Go API → production Next.js → Chromium stack, and compared the rendered product with the unchanged canonical HTML reference. The earlier evidence above is the implementation checkpoint; this section records the audit and its fixes. No Phase 05 provider, LLM integration, backend behavior, source data, commit or push was introduced.

### Findings And Corrections

No critical security or analytics-integrity defect was found. Important Phase 04 defects corrected:

1. Mobile table headers were visually hidden, taking the only sorting controls with them. Mobile now has a labelled sort selector and direction control; desktop keeps sortable headers. The real API verifies every sort in both directions.
2. A failed analysis-status request was discarded by the dashboard while polling continued. After normal retries are exhausted the error is visible, polling stops, prior results remain visible, and **Retry status** fetches the same run without another analysis POST.
3. Meter detail fetched finding evidence but omitted numeric baseline/observed energy. It now displays the stored comparison for flagged hours, with loading and retry states; no baseline is computed in the browser.
4. Long identifiers and large consumption values overflowed the mobile table by **326 px**. Text, numeric cells, badges and data-dependent action links now wrap; a labelled synthetic layout probe checks the table and investigation at 390 px without substituting API data in the acceptance flow.
5. A future `recommended_action` value could produce an undefined icon and crash the investigation. Unknown type, severity, action and status values retain a neutral/readable fallback rather than inventing a known classification or successful run.
6. Narrowing the browser network monitor exposed an acknowledged logout request being aborted during navigation. The central client now finishes the 204 response before navigating; twenty successive sign-in/out cycles check for false expiry, failures and redirect races.

Smaller corrections: failed login clears the password and restores focus with an associated error description; placeholders use the readable muted token (**5.59:1**, previously **2.53:1** on white); the chart tooltip uses the inverse-text token instead of a literal color; full-page error/not-found states have an h1; failed session-detail loading has an explicit retry. The browser-test allowlist now identifies exact HTTP method/path/status combinations, allowing only intentional GET cancellations for API requests and RSC navigation. Harness cleanup closes log descriptors, waits for owned child processes, handles SIGTERM, bounds readiness requests and verifies the temporary-directory boundary; failure messages omit command arguments that could contain generated credentials.

### Repository, Contracts And Boundaries

- The original staged deletion of `src/frontend/.gitkeep` is preserved. The unstaged `tests/e2e/.gitkeep` deletion belongs to TD-29. The frontend source, test harness, configuration and lockfile are Phase 04 additions; README, phase, design, testing, decisions, traceability and instruction-router edits document that scope. No unrelated tracked changes were identified or altered. `.next`, `node_modules`, test screenshots/traces and scratch audit artifacts remain ignored.
- App Router server layouts check cookie presence; the API validates the session. Browser I/O remains centralized in `lib/api/client.ts`, with same-origin credentials and no-store requests. No browser token storage, client-side secrets, analytics recomputation, dataset-ID branching, or Phase 05 integration was found.
- OpenAPI generation was run twice; both outputs equal the initial SHA-256 `4EF6A0379219478FD376DBBFEE5F9A984431CCC0F8B721E2DBF5B7652C0D9C47`. No generated declarations were manually edited. The only API-contract diff is the two quoted descriptions above.
- `BACKEND_URL` is a production **build-time** setting: the concrete destination is in `.next/routes-manifest.json`, and the installed Next.js router reads that manifest. The rewrite has a fixed configured destination, not a user-supplied proxy target. README now distinguishes production builds from development startup.
- Source CSV Git blob hashes still equal HEAD: readings `1c1a11a7aef21a75e4649d8e71f52747b1cf8816`, events `d6a23acd4b6fbb0600f4fa835fe46bbd9944e426`. The reserved evaluator data was not accessed.

### Behavioral And Visual Acceptance

The fresh database shows 12 meters and 155,250.85 kWh with unanalysed KPI placeholders and meter statuses. The first real analysis produces 4 findings, 2 high priority, 83% aggregate confidence, and fleet status 1 Critical / 2 Alert / 9 OK. A separate component fixture proves that completed-with-zero-findings is distinct from never analysed. Rapid repeated clicks, an immediately completed run, active-run restoration, failed analysis, monitoring failure, retry and terminal polling are tested without fabricating an in-progress state in the real journey.

Real browser checks cover anonymous access to all protected route families, both invalid credential fields, cookie tampering, twenty consecutive session cycles, literal meter searches (`M-109`, `m-10`, missing ID, `%`, `_`, Unicode), all status/severity/type filters, both sort directions, 10+2 pagination, and missing resources. The real API is stopped and restarted after the main suite; the dashboard safely displays the proxy failure, Retry restores its data, and Retry session restores the username.

Every one of the **336 readings × 4 metrics** is compared with the real API in browsers configured for **Pacific/Kiritimati and America/Los_Angeles**. Table timestamps preserve source order and wall time, including the 14:00 onset. Source-axis and tooltip formatting, exact-time event/episode markers, and baseline values from stored signals have focused tests; the browser does not infer or interpolate an analytical baseline. The ECharts instance stays the same when changing metrics; its observer, resize and disposal lifecycle has a component regression test.

All four investigations expose their stored classification, severity, confidence, evidence and action. M-109's UNKNOWN event stays context-only; M-112 separates measurement inconsistency from stable consumption; M-104 remains an explainable deviation; M-106 remains a detected, explained and recovered false positive with no escalation. M-101 has no current finding. Confidence is expressly evidence strength, not failure probability. Technical evidence starts collapsed and API-supplied HTML-like text remains plain text.

Screenshots cover login, dashboard, meters, anomalies, meter detail and investigation at **1440 / 768 / 390 px**, plus pre-analysis, missing-resource, actual-outage and synthetic-long-content states. Compared with the rendered reference: navy shell/action hierarchy, loaded Plus Jakarta Sans font, white panels, rounded controls, semantic badges, spacing and mobile stacking remain coherent. Shapes and text distinguish states. Independent token contrast measurements are normal 4.82, informational 9.95, warning 4.98, critical 7.03, data quality 5.55 and neutral 5.12 to one. Keyboard checks cover login submission/error recovery, skip link, filters, mobile-sheet focus containment, Escape/focus return and scroll unlock. No automated axe or screen-reader certification is claimed.

### Traceability Decision

**FR-INV-001: Verified (option B).** All seven required investigation elements are visible with backend evidence; the requirement does not require an LLM or explanation-provider abstraction. **FR-UX-001: Verified (option B).** The canonical visual language and complete login → meter → analysis → investigation → explanation → action path work against the real stack. Phase 05 remains a separate enhancement for providers/grounding, and Phase 06 owns delivery/CI and broader accessibility/browser hardening. Neither is needed to declare these two existing acceptance criteria verified.

### Final Audit Evidence

| Check | Final command / result |
| --- | --- |
| Frozen dependency state | `pnpm install --frozen-lockfile`; forced offline reinstall also passed; `pnpm audit`: no known vulnerabilities |
| Lint / types | `pnpm lint`; `pnpm typecheck`: passed |
| Unit + component | `pnpm test`, then complete reruns with `TZ=Pacific/Kiritimati` and `TZ=America/Los_Angeles`: **14 files / 71 tests** each, no failures or skips |
| Clean production build | `.next` moved to an ignored audit backup, then `pnpm build`: **8 routes**, no warnings or ignored errors |
| Real-stack browser | `pnpm test:e2e` twice: **10/10 tests** each plus actual API stop/restart recovery; disposable PostgreSQL 18.6, compiled Go commands, production Next.js, Chromium, no API mocks |
| Backend regression | `go -C src/backend test -json -count=1 ./...`: **154 top-level + 89 subtests**; integration variant: **199 + 124**; no failures/skips |
| Backend static / generated | `go -C src/backend build ./...`; `go -C src/backend vet -tags=integration ./...`; pinned sqlc `diff`: passed |
| Contracts / repository | OpenAPI types generated twice with identical hash; `git diff --check` passed; source CSV hashes match HEAD; scoped production scans found one centralized `fetch`, no browser credential storage, analytics recomputation, provider integration or dataset-specific production branches |

Final audit status: **VERIFIED COMPLETE**. Phase 04 is ready for the Phase 04 commit. Phase 05 remains unauthorized and was not started.

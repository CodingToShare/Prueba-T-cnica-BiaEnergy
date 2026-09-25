# Design System

The visual and interaction contract for the Bia Energy Management Platform frontend.

## 1. Visual Reference vs Product Implementation Contract

| | Visual reference | Product implementation contract |
| --- | --- | --- |
| Artifact | [`docs/design/reference/design-system-v3.html`](reference/design-system-v3.html) | This document |
| Role | Canonical visual sample: the look and feel to match | The binding rules for the Energy Management product |
| Defines | Palette, typography, spacing, radii, shadows, shell/sidebar, KPI cards, buttons, fields, cards, badges, alerts, tables, hover/focus/active/disabled behavior | Semantic tokens, Energy Management meanings (severity, classification, status), charts, states, accessibility, responsive rules, validation |
| Does **not** define | Product requirements, business terminology, domain entities, data, authentication, navigation items | — |

The reference is the **Bia Energy edition** of design system v3: it keeps the original v3 visual language and shows it applied to this product (dashboard, meters, investigation queue, investigation with evidence and chart, states), with accessibility and UX improvements listed in its "Cambios respecto a la v3 original" section. Its Spanish copy and sample values are **illustrative** (taken from the challenge examples or approximated by hand); they are not engine output, production data, or requirements, and they do not decide the UI language (OD-13).

The reference changes only through an explicit design decision recorded in the Phase document that makes it, and it must stay consistent with this contract. When the two differ, this document wins for the product; the reference remains the comparison baseline for visual quality.

## 2. Visual Direction (Adopted From The Reference)

- **Typography:** Plus Jakarta Sans (400–800) with Inter and system sans-serif fallbacks. Headings are heavy (700–800) with slight negative tracking; labels are small, bold, and uppercase only in table headers and group labels. Metrics use tabular numerals.
- **Primary visual language:** navy as the single primary interactive color (reference `#1E3A5F`, with hover `#16314F`, active `#0F2238`, soft `#EAEFF5`, and a translucent focus ring).
- **Supporting accents:** mint, sky, violet, amber — for hierarchy, charts, and semantic roles assigned below; never as alternative primary actions.
- **Surfaces:** light neutral page background, white content surfaces, muted surface for table headers and panels, subtle borders, restrained layered shadows, generous spacing, rounded but professional radii (reference scale 10 / 14 / 20 / 26 px).
- **Shell:** navy sidebar with grouped navigation and a clearly active item; content area on a muted surface; top bar with page title and primary action.

Reference values are the starting token values. Components consume **semantic tokens**, never the raw values, so branding can evolve without touching components.

## 3. Semantic Tokens

Defined once as CSS variables, mapped into Tailwind, shadcn/ui theming, and the ECharts theme (Phase 04).

| Group | Tokens | Initial value source |
| --- | --- | --- |
| Surface | `background`, `surface`, `surface-muted`, `border`, `border-strong` | Reference neutrals |
| Text | `text-heading`, `text`, `text-muted`, `text-subtle`, `text-inverse` | Reference neutrals |
| Action | `primary`, `primary-hover`, `primary-active`, `primary-soft`, `focus-ring` | Reference navy |
| Status | `normal`, `informational`, `warning`, `critical`, `data-quality` — each with `-fg`, `-soft`, `-border` | See §4 |
| Destructive | `destructive` (`#DC2626`, white text 4.83:1), `destructive-text` (`#B91C1C`), `destructive-soft` | Reference danger |
| Chart | `chart-series-1…5`, `chart-baseline-band`, `chart-anomaly-region`, `chart-event-marker`, `chart-data-quality-marker`, `chart-grid`, `chart-axis` | Navy + accents |
| Shape | `radius-sm/md/lg/xl`, `shadow-xs/sm/md/lg` | Reference |

No other color values may appear in components. Final values are confirmed in Phase 04 with WCAG AA contrast checks; a value may be adjusted for contrast while keeping its hue family.

## 4. Energy Management Semantics

Five semantic states, used identically in badges, alerts, table rows, KPI context, and charts:

| Semantic | Meaning | Text / soft background | Contrast | Badge shape |
| --- | --- | --- | --- | --- |
| `NORMAL` | Healthy, expected behavior | `#0F766E` / `#E3F4F1` (teal) | 4.82:1 | circle |
| `INFORMATIONAL` | Contextual or operationally explained condition | `#1E3A5F` / `#EAEFF5` (navy) | 9.95:1 | circle |
| `WARNING` | Needs review, not immediate escalation | `#8A6100` / `#FCF2DD` (amber) | 4.98:1 | triangle |
| `CRITICAL` | High-priority real anomaly | `#9F1239` / `#FDECEF` (rose) | 7.03:1 | diamond |
| `DATA_QUALITY` | Measurement reliability or consistency issue | `#4F46E5` / `#EEF0FE` (indigo) | 5.55:1 | ring |
| Neutral | Not analyzed yet, unknown, low severity | `#5D6880` / `#F3F5FA` | ≥ 4.8:1 | dashed ring |

These values were derived from the original v3 semantic colors, which failed WCAG AA text contrast (3.3–4.2:1). They were darkened within their hue families and chosen so that the four chromatic states stay distinguishable under deuteranopia and protanopia (OKLab ΔE ≥ 8 for all pairs; warning vs critical was ΔE 0.5 in the original). `text-muted` moved from `#66718A` to `#5D6880` for the same reason. Shapes provide the non-color cue.

### Three separate concepts

Severity, anomaly type, and destructive actions are different concepts with different token groups:

| Concept | Values → semantic |
| --- | --- |
| Anomaly type (classification) | `REAL_ANOMALY` → `CRITICAL` when severity is high, otherwise `WARNING`; `EXPLAINABLE_ANOMALY` → `INFORMATIONAL`; `FALSE_POSITIVE` → `INFORMATIONAL` (visually quiet); `DATA_QUALITY` → `DATA_QUALITY` |
| Severity | `HIGH` → `CRITICAL`; `MEDIUM` → `WARNING`; `LOW` → neutral |
| Meter computed status | OK → `NORMAL`; Alert → `WARNING`; Critical → `CRITICAL`; not analyzed → neutral |
| Destructive action / error | `destructive` only — delete/reset buttons and failure messages |

Rules:

- `critical` and `destructive` are separate tokens even if they start from the same red family; components never reference one for the other's purpose.
- Destructive/danger styling is never used merely because something is important.
- A data-quality finding is never shown in the critical color for its type; its severity badge carries severity separately.
- Consumption direction is not good/bad: KPI deltas (up/down) use neutral or informational styling, never success/danger.
- Color is never the only carrier of meaning: badges keep the reference dot-plus-text form and add an icon where it helps; confidence shows value plus band (e.g., "0.96 · High").

## 5. Components

Reuse the reference patterns through shadcn/ui primitives themed by tokens:

- **Buttons:** primary, secondary (soft navy), outline, ghost, destructive; sizes sm/md/lg; visible hover, active, focus ring, disabled. One dominant primary action per screen (e.g., Run AI Analysis).
- **Fields:** explicit labels above inputs, hint text, error state with message, focus ring.
- **KPI cards:** label, value with unit, context line. A featured navy tile is optional and limited to one per view.
- **Tables:** uppercase muted headers, row hover in primary-soft, sortable headers with direction indicator, and a stacked-row layout on small screens.
- **Badges and alerts:** reference forms with semantic tokens; alerts use role `alert`/`status` appropriately.
- **Panels/cards:** frame self-contained content; not the default wrapper for every section. Card hover lift only on clickable cards.
- **Progress:** analysis progress uses the reference progress bar with a step list naming the current stage.

- **Filters and sorting:** segmented control with `aria-pressed` for status filters; sortable headers expose `aria-sort`.
- **Investigation queue:** numbered, priority-ordered list with type, severity, confidence, and the recommended action as a link.
- **Explainability block:** what happened → why it matters → evidence (definition list) → what to do, plus confidence components as labeled bars.
- **States:** skeleton loading, empty ("no analysis yet" with the primary action), no-results (with "clear filters"), error (with retry).

The only gradient is the navy sidebar; logo, featured tile, and progress bar are solid navy or accent.

## 6. Charts

- ECharts theme generated from tokens: series colors from navy and accents, shared typography, `chart-grid`/`chart-axis` neutrals, consistent tooltips and number/unit formatting.
- Time series show the observed values, the baseline band, anomaly regions (`critical-soft`/`warning-soft`), operational event markers (`chart-event-marker`), and data-quality markers (`chart-data-quality-marker`), each labeled.
- Consumption, voltage, current, and power factor share one time axis when shown together.
- Charts must stay readable at small widths (fewer ticks, stacked panels) and provide a text summary of the key finding.

## 7. States And Accessibility

- Every async view designs loading (skeletons), empty, recoverable error with retry, disabled, and "not analyzed yet" states.
- WCAG 2.1 AA contrast; visible focus (reference focus ring); full keyboard operation; semantic landmarks; labeled controls; reduced-motion support.
- Data density stays readable: prioritize the question the screen answers; polish never compensates for weak information hierarchy.

## 8. Responsive Behavior

Desktop keeps the sidebar shell. At ≤ 900 px the sidebar becomes a top bar with an explicit, keyboard-accessible menu button (`aria-expanded`). KPIs use 3 columns, then 2 at ≤ 720 px. At ≤ 640 px tables become stacked, labeled rows. No accidental horizontal page overflow at any width.

## 9. Visual Validation Policy

Lightweight and review-based, not pixel-perfect:

- For each significant product screen, capture or review representative views at approximately **1440 px** (desktop), **768 px** (tablet), and **390 px** (mobile). Exact viewports may vary slightly.
- Validate information hierarchy, typography, spacing, surfaces, forms, buttons, table/list behavior, charts, navigation, states, responsive layout, and absence of horizontal overflow.
- Compare side by side with the reference HTML for visual language (not content).
- Playwright screenshots may serve as evidence and are stored with the phase evidence when useful.
- No brittle pixel-diff assertions unless a concrete regression justifies one.

## 10. Implementation (Phase 04)

Where the contract lives in code (`src/frontend`):

- **Tokens:** `app/globals.css` declares the reference values as CSS variables on `:root` (surfaces, text, navy primary, focus ring, the six status tones with their `-soft` backgrounds, chart colors, radii, shadows) and maps them into Tailwind v4 with `@theme inline` (`bg-surface`, `text-heading`, `text-critical`, `rounded-md`…). shadcn/ui variables (`--primary`, `--border`, `--ring`…) are bound to the same tokens, so primitives and product patterns share one palette. Font: Plus Jakarta Sans via `next/font` (400–800).
- **Reference patterns as CSS component classes** in `globals.css`: `.badge` + `.badge-{normal|informational|warning|critical|data-quality|neutral}` (each tone also has a distinct glyph: dot, triangle, diamond, ring, dashed ring, so color is never the only carrier), `.alert-*`, `.kpi` / `.kpi-feature`, `.panel`, `.tbl` (with `.tbl-stack` for stacked labeled rows at ≤ 640 px via `data-label`), `.segmented`, `.queue`, `.progress-track`, `.steps`, `.story`, `.evidence-list`, `.conf-row`, `.state`. React wrappers: `components/badges.tsx`, `components/layout.tsx` (PageHeader, Panel, KpiCard, MeterBar), `components/states.tsx`, `components/segmented.tsx`.
- **shadcn/ui primitives** (copied into `components/ui`, restyled): Button (primary, secondary, outline, ghost, destructive; sm/md/lg/icon), Input, Label, Skeleton, Sheet (mobile navigation).
- **Semantic mapping** (`lib/format/labels.ts`): type → tone (real HIGH critical, real otherwise warning, data quality data-quality, explainable and false positive informational); severity → tone; computed status → tone (`null` = neutral "Not analyzed"); event role → tone (`EXPLAINS` informational, `CORROBORATES` data-quality, `CONTEXT` neutral "Context only — does not explain it").
- **Shell:** navy gradient sidebar above 900 px; at ≤ 900 px a navy top bar with a "Menu" button opening a Radix sheet (focus trapped, `aria-expanded`, Escape closes). Skip link to `#main`. Dashboard KPIs: 3 columns, 2 at ≤ 720 px. Meter-detail KPIs: 4 columns from 1024 px, otherwise 2.
- **Charts** (`features/meters/chart`): ECharts core with only the line chart, grid, tooltip, mark-area and mark-line modules, loaded on demand in the browser. Colors, grid and font come from the CSS tokens at runtime. The category axis is the list of source timestamps, so no time zone conversion happens. Overlays come only from stored evidence: the finding's episode as a dashed critical-soft region, correlated events as vertical markers at their exact time, and the engine's baseline at the flagged hours as a dashed line. A legend names each overlay, the figure has an accessible text summary, and "View data as table" provides the values.
- **Deviations from §6, recorded:** there is no continuous baseline band, because the API exposes the baseline only for flagged readings and the browser must not recompute it. A full band needs a backend baseline series (deferred). Data-quality events use the same event marker as other events; their role badge in the investigation carries the meaning. Metrics are shown one at a time with a segmented selector instead of stacked shared-axis panels, which keeps the chart readable at 390 px.

Visual review (Phase 04): the desktop 1440 px, tablet 768 px and mobile 390 px screens were captured by the real-stack Playwright run and compared with the reference for hierarchy, typography, spacing, surfaces, badges, tables, navigation and states. `docs/design/reference/design-system-v3.html` was not modified.

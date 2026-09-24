# Phase 00 — Repository + AI Agent + Engineering Governance Foundation

- Status: **Complete** (after corrective validation)
- Authorization: explicitly authorized by the user (Phase 00 only), including a corrective and hardening pass.
- Date: 2026-09-24

## Objective

Turn a scaffold reused from an unrelated earlier challenge into a clean, coherent, agent-ready engineering workspace for the Bia Energy Management Platform, with canonical visual and testing governance, without implementing the application.

## Non-Goals (Respected)

No Go module, Next.js app, migrations, dependencies, APIs, repositories, algorithms, UI, authentication, Docker Compose, CI, LLM calls, prompts, synthetic data, or challenge answers were created. Phase 01 was not started.

## Inspection Findings

- The workspace held governance for an unrelated earlier product and stack: domain requirements, 6 ADRs, 8 phase records and a roadmap, a governance audit, a database performance review, screenshots and a browser acceptance script, stack-specific skills, an SDK pin file, and empty test project folders.
- Reusable concepts retained: `AGENTS.md` router with category-specific authority; separation of requirements, rules, decisions, assumptions, and scope; explicit phase authorization with evidence-based completion; Definition of Done; `Planned` traceability; skill format; risk-based testing; the canonical HTML visual reference.
- The previous `.gitignore` treated agent files and `docs/ai/` as local-only; replaced so shared governance is tracked (TD-14).
- The workspace was not a Git repository; `git init` was run (no commits).
- `readings.csv` and `events.csv` were not on disk. Only `data/input/` and its README were created; nothing was generated or transcribed. Dataset facts in `docs/ai/anomaly-analysis.md` come from inspecting the supplied challenge material and must be confirmed in Phases 01–02.

## Corrective Pass

### Design reference incident and recovery

The first pass deleted `docs/design/reference/design-system-v3.html` as a legacy artifact. The corrective instruction established it as the canonical visual reference. Because the workspace had no Git history, the file was recovered from the previous challenge's Git repository on the same machine, where it is tracked and unmodified at its HEAD commit. A second copy of that project had identical content (differing only in line endings). The restored file was byte-identical to the tracked source (SHA-256 `04fe23fc…d8ab548`, 507 lines). The original v3 remains available in that repository's history.

### Design reference adaptation (explicit user request)

The user then asked to replace the previous product's business content in the reference and to apply pertinent UI/UX improvements. The HTML was rebuilt as the **Bia Energy edition of v3**, keeping the v3 visual language (navy `#1E3A5F` primary, Plus Jakarta Sans, tokens for surfaces, radii, shadows, and component anatomy):

- All previous business content (brand, condominium units, residents, gatehouse, packages, balances) replaced with product content: dashboard with the six challenge KPIs, prioritized investigation queue, meters table with status filter and sort, M-109 investigation (explainability block, evidence, confidence components, baseline chart with anomaly period, table view), analysis progress by stage, empty/loading/no-results/error states. Values are illustrative and labeled as such.
- Semantic colors: the original v3 badge colors failed WCAG AA (3.27–4.23:1). They were replaced by five Energy states measured at 4.82–9.95:1, validated for deuteranopia/protanopia separation (worst pair ΔE 8.2, originally 0.5 for warning vs critical). Badges gained shapes, so color is not the only cue. `critical` is separate from `destructive`.
- Accessibility: visible focus on all interactive elements, skip link, labeled search and fields with `aria-describedby`/`aria-invalid`, `aria-current`, `aria-pressed`, `aria-sort`, `role="status"`/`alert`, `prefers-reduced-motion`.
- Responsive: the original hid the sidebar below 720 px with no alternative. Now the sidebar becomes a top bar with a menu button at ≤ 900 px, KPIs go from 3 to 2 columns, and tables stack into labeled rows at ≤ 640 px.
- Removed: multi-hue gradients, notification bell/avatar, and red/green for KPI up/down.

The HTML is authored content, so the byte-for-byte rules were removed from `.gitattributes`/`.editorconfig`. `docs/design/design-system.md`, TD-15, `AGENTS.md`, project context, the UI skill, and the roadmap were updated to match.

Design validation: rendered with headless Chrome at 1440 px, and at 768 and 390 px through a fixed-width iframe (desktop Chrome enforces a ~500 px minimum window). Layout issues found and fixed: KPIs wrapped as 4 + 2; the two-column table/queue split clipped columns; the tablet sidebar hid table columns; two chart annotations collided; chart labels used a status color instead of text ink. Final renders show no horizontal overflow at any of the three widths.

### Governance hardening

- Design: `docs/design/design-system.md` rewritten as the product implementation contract that references and adapts the HTML (navy primary, Plus Jakarta Sans, reference tokens) with Energy semantics (normal, informational, warning, critical, data-quality), separating severity, anomaly type, and destructive actions; visual-validation policy at ~1440/768/390 px.
- Testing: continuous-testing principle; unit, integration, functional levels; LLM testing without live models; regression rule; deterministic data; clean-environment validation.
- Phase governance: Phase Entry Gate (baseline validation evolving per phase, "Blocked by baseline regression"), Phase Exit Gate, eight roadmap gates, Implement → Validate → Review → Document → Complete.
- Definition of Done: 20 categories with N/A only by reason.
- Phase 01–06 documents: per-phase expected validation, data-quality items (01/02), API quality items (03), product/UX items (04/05), reproducible demo (06).
- Traceability: planned test levels per requirement; new rows for analysis status, M-109 prioritization, recommended action, responsive UX, reproducible startup.
- New register entries: FR-DEL-002 (reproducible demo), TD-15 (visual reference), TD-16 (continuous testing), OD-19 (automation entry point).

## Files

| Action | Items |
| --- | --- |
| Deleted (first pass) | 6 legacy ADRs; 8 legacy phase records + old roadmap; governance audit; database performance review; 6 screenshots, acceptance script, old agent context under `docs/ai/`; 3 stack skills; SDK pin file; legacy test and database script folders |
| Restored, then rebuilt as Bia Energy edition | `docs/design/reference/design-system-v3.html` |
| Rewritten | `AGENTS.md`, `CLAUDE.md`, `.github/copilot-instructions.md`, `README.md`, `.editorconfig`, `.gitignore`; skills `architecture`, `code-review`, `documentation`, `phase-governance`, `testing`, `ui-design-system`; `docs/product/*`; architecture; design system; Definition of Done; testing strategy |
| Replaced skills | `nextjs-frontend`, `go-backend`, `postgresql` |
| Created | `.agents/context/project-context.md`; skills `analytics-engine`, `ai-explainability`, `data-ingestion`, `observability`; ADR-001…007; `docs/ai/anomaly-analysis.md`, `docs/ai/explainability.md`; roadmap + phases 00–06; `docs/performance/data-query-strategy.md`; `data/input/README.md`; `.gitattributes`; `.gitkeep` placeholders |

## Evidence (Corrective Validation)

| Check | Result |
| --- | --- |
| Reference HTML exists | Yes; Bia Energy edition (restored first, identical to the tracked source, then rebuilt at user request) |
| Reference content | No previous-product business terms remain; values marked illustrative; the design contract states it is not a source of requirements |
| Reference contrast and colorblind separation | Semantic text colors 4.82–9.95:1; four chromatic states pass deuteranopia/protanopia ΔE ≥ 8 (palette validator) |
| Reference responsive render | 1440 / 768 / 390 px screenshots reviewed; no horizontal overflow |
| Design contract, UI skill, testing strategy, `AGENTS.md`, project context, roadmap, DoD reference the HTML | Yes (each file checked) |
| Visual validation policy (~1440/768/390) in design system, testing strategy, UI skill | Yes |
| Entry and Exit Gates in phase-governance skill, roadmap, DoD, `AGENTS.md` | Yes |
| Testing skill free of previous-project rules | Yes |
| Roadmap phases | 00–06 |
| Phase 06 described as hardening/regression, not first-time testing | Yes |
| DoD includes unit, integration, functional/E2E | Yes (§4–§6) |
| Traceability has planned test levels | Yes |
| Obsolete screenshots and acceptance script | Absent (no `*.png`, `*.cjs`, `phase8*` in the tree) |
| Legacy scan (previous product, entities, fields, stack, phase identifiers, brand, old status/priority semantics) excluding the reference HTML | No matches |
| Legacy terms inside the reference HTML | None |
| Application code (`*.go`, `go.mod`, `package.json`, `*.ts(x)`, `*.sql`, compose/Docker/YAML, CSV) | None |
| `git status` / `git check-ignore` / `git check-attr` | See final report; ignore and attribute rules verified; reference HTML and CSVs marked non-text |

`git diff --stat` is not applicable: there is no baseline commit. The Files table is the change summary.

## Known Limitations

- Source CSVs must be placed in `data/input/` by a person before Phase 01.
- No commit was created; committing is left to the repository owner.

## Deferred

OD-01…OD-19 in `docs/product/assumptions-and-decisions.md`.

## Next Phase

None authorized. Phase 01 requires explicit user authorization and must begin with its Phase Entry Gate.

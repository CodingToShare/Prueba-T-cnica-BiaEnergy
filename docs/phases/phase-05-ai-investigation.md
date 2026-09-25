# Phase 05 — AI Investigation / Explainability Integration

- Status: **Complete** (independently audited; implementation uncommitted)
- Authorization: explicitly authorized by the user, together with the Phase 04 checkpoint commit. Phase 06 is not authorized and was not started.
- Date: 2026-09-25
- Phase 04 checkpoint: local commit `699da3f` "feat: build energy management frontend experience" (author Santiago Forero, no co-author trailer; not pushed; no remote, no tag).

## Objective

Add a grounded explanation layer to the investigation while the deterministic engine stays the only source of analytical decisions. The pipeline is `Analytics engine → Structured evidence → ExplanationProvider (Deterministic | Ollama) → Persisted explanation → API → Investigation UI`.

## Phase 04 Checkpoint

Before committing, the audited tree was reviewed (`git status`, staged and unstaged diffs, log). The staged deletion of `src/frontend/.gitkeep` and the unstaged deletion of `tests/e2e/.gitkeep` were both kept. The untracked set was 93 files; the only additions since the implementation inventory were the audit's own tests (`e2e/audit.spec.ts`, `e2e/api-recovery.mjs`, `features/meters/chart/readings-chart.test.tsx`) and `app/icon.svg`.

Regression on that tree:

| Area | Result |
| --- | --- |
| Frontend | `pnpm install --frozen-lockfile`, lint, typecheck; Vitest 14 files / 71 tests; build |
| Backend unit | 154 top-level + 89 subtests |
| Backend integration | 199 + 124 |
| `git diff --check` | Clean |

There were 0 failures and 0 skips. 105 paths were staged (10 modified, 2 deleted, 93 added). None was `.env`, `node_modules`, `.next`, a test artifact, a screenshot or `expected_results.csv`, and the secret scan of the staged diff was clean. Committed as `699da3f`, leaving a clean tree.

## Phase Entry Gate

| Check | Result |
| --- | --- |
| Previous phase | Phase 04 independently verified and committed (`699da3f`) |
| Baseline | The regression above, on the same tree |
| Context read | AGENTS.md, project context, the phase-governance, architecture, go-backend, ai-explainability, testing, code-review, documentation and ui-design-system skills, ADR-006, `docs/ai/*`, architecture, OpenAPI, FR/BR, traceability, testing strategy, Definition of Done |
| Canonical model | The Phase 02 `analysis.Finding` and the Phase 03 persisted `analysisrun.Evidence` are reused as-is; no parallel evidence model |

## Implementation

| Area | What was built |
| --- | --- |
| Boundary | `analysisrun.ExplanationProvider { Explain(ctx, ExplanationInput) (Explanation, error) }`, declared by its consumer. `ExplanationInput` is one finding's conclusions plus its persisted `Evidence`; `Explanation` is four texts plus source, model and prompt version |
| Deterministic provider | `internal/explanation/deterministic.go`: templates per classification over the evidence (`evidence-template-v1`); no thresholds or meter-specific wording; about 2.6 µs per finding (benchmark) |
| Ollama provider | `internal/explanation/ollama.go`: standard-library `POST /api/chat`, JSON-schema `format`, temperature 0.2, `num_predict` 600, per-call `OLLAMA_TIMEOUT`, 64 KiB body limit; failures typed with sanitized codes |
| Prompt | `prompt.go` (`energy-explanation-v1`). Instructions live only in the system message; the evidence is a JSON user message; source text is data. Contract: `docs/ai/prompts/energy-explanation-v1.md` |
| Validation | `validate.go`: strict JSON with exactly four unique string fields, rune-based length limits (320/700), no markup or control characters, no model-authored numbers, known event types presence-checked and causal roles guarded. `why_it_matters`, `evidence_narrative` and `recommended_action_text` must equal controlled deterministic values; only the qualitative summary is model-authored |
| Orchestration | New real stage `GENERATING_EXPLANATIONS` (progress 50) between the engine and persistence; sequential; fallback to the deterministic text with a code; stage budget (3 min with Ollama, added to the run timeout); only run cancellation stops it |
| Configuration | `EXPLANATION_PROVIDER` (default `deterministic`), `OLLAMA_BASE_URL` (default `http://127.0.0.1:11434`, http(s), no credentials), `OLLAMA_MODEL` (required for Ollama), `OLLAMA_TIMEOUT` (default 60s, ≤ 5m); validated at startup, secrets never echoed |
| Persistence | Migration `00003_add_finding_explanations.sql`: `anomalies.explanation` (JSONB) plus source, model, prompt version, generated_at, fallback_used and fallback_code, with consistency constraints; `analysis_runs.explanation_configuration` (JSONB, no URL); stage constraint extended. The down migration is lossless for analytics |
| API | `GET /api/v1/anomalies/{id}` returns `explanation` (`source`, `summary`, `why_it_matters`, `evidence_narrative`, `recommended_action_text`, `model`, `prompt_version`, `generated_at`, `fallback_used`), or `null` for pre-Phase-05 findings; new stage in `AnalysisRun`. OpenAPI updated; the DTO/schema consistency test covers `Explanation` |
| Frontend | Regenerated types (two generations, identical SHA-256 `b00f5db9…2e10`). New "Writing explanations" step (6 steps). New `ExplanationPanel`: provenance label, transparency sentence, collapsed details. The action card uses `recommended_action_text`; the static "Why it matters" is omitted when an explanation exists. No regenerate button, no raw HTML |
| Logs | `explanation provider configured`, `explanation generation started`, `explanation generated`, `explanation fallback used`, with analysis id, priority, meter, provider, model, duration and fallback code. No prompts, responses, event descriptions or raw errors |

Not added: vector DB, embeddings, RAG, LangChain, agent frameworks, MCP, hosted AI SDKs, streaming, websockets, an AI job queue, a prompt database, token accounting, or a generic multi-provider framework. No dependency was added: the Ollama client uses `net/http`.

## Validation Evidence

| Gate | Command | Result |
| --- | --- | --- |
| Formatting / modules | `gofmt -l` (tracked and new files), `go mod tidy` (no diff), `go mod verify` | Clean / all modules verified |
| Build / static | `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, pinned sqlc `diff` | Clean |
| Backend unit | `go -C src/backend test -json -count=1 ./...` | **183 top-level + 117 subtests**, 0 failed, 0 skipped |
| Backend integration | `go -C src/backend test -json -count=1 -tags=integration ./...` | **237 + 157**, 0 failed, 0 skipped (PostgreSQL 18.6 via testcontainers) |
| Repeat / shuffle | `go test -count=3 ./internal/explanation ./internal/analysisrun`; unit and integration suites with `-shuffle=on -count=1` | Passed; no timing failure |
| Race (Linux race image + Docker CLI, repo mounted read-only) | `go test -race -count=1 ./...` and `-tags=integration` | Every package passed; 0 `DATA RACE` reports |
| Frontend | `pnpm install --frozen-lockfile`, `pnpm generate:api` ×2, `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` | Identical generations; clean; **14 files / 81 tests**; build OK |
| Real-stack E2E | `pnpm test:e2e` | **10/10 Playwright tests**, plus the API-outage recovery and the Ollama-unavailable fallback paths; no mocks; containers removed |

New and changed tests:

- **Explanation package**: deterministic, adapter, validation and PostgreSQL orchestration tests.
  - `deterministic_test.go` (8): the four classifications; context-only events are never an explanation; event descriptions are not copied; no events or an unknown type; the text passes grounding.
  - `ollama_test.go`: success and request shape; prompt injection stays data; a hostile reply with authoritative fields is rejected; the payload has no secrets or signals; strict duplicate/trailing/type handling; response-size and Unicode boundaries; redirects; refused connection; timeout; cancellation; model-authored numbers; event-role, consequence, evidence and action contradictions; controlled text accepted; configuration and settings.
  - `orchestration_integration_test.go` (7, real PostgreSQL):
    - A: deterministic run with round trip through the anomaly service.
    - B: Ollama success with model and prompt persisted, 4 sequential requests, no URL in the snapshot; a mixed success/HTTP-error/success/timeout run proves per-finding provenance is independent.
    - C–E: unreachable, malformed, HTTP 500, invented number and timeout all fall back with the right code, the run COMPLETES, and outcomes are identical to the deterministic run.
    - Stage budget.
    - F: analytics failure → FAILED with no provider call.
    - No text available → finding stored with `explanation = null`; reading findings never invokes a provider; parent cancellation invokes no fallback.
    - Schema constraints.
- **Black-box**: the golden path asserts the deterministic explanation and that descriptions are not logged. `TestBlackBox_OllamaUnavailable_AnalysisCompletesWithDeterministicFallback` runs the compiled API with Ollama at a refused address: COMPLETED, the four findings unchanged, every explanation a fallback with `model = null`, the code not in the API, `provider_unavailable` in the logs, no raw errors or secrets.
- **Updated**: `model_test.go` (new stage; `FallbackCode`), `config_test.go` (2 tests), the migration tests (up 1–3, down 3 with a run in the new stage, down 2, re-up), and in-package orchestration tests using a stub provider.
- **Frontend**: `investigation-view.test.tsx` gains 10 explanation tests:
  - Ollama provenance and transparency copy, with no live region;
  - collapsed details (source, model, prompt version, fallback);
  - deterministic label without a model;
  - fallback shown as a label, not an alert;
  - action wording from the explanation;
  - `<script>alert(1)</script>` and `<b>` rendered as text;
  - missing model;
  - unknown future source;
  - maximum-length text;
  - legacy `null`.

  `state.test.ts` and the dashboard tests cover the sixth step.
- **E2E**:
  - The golden path checks "Evidence-based explanation" and the transparency copy on M-109.
  - `explanation-fallback.mjs` restarts the real API with `EXPLANATION_PROVIDER=ollama` at a free (closed) port. It re-runs the analysis to COMPLETED ("4 anomalies detected · 2 require high-priority attention"), checks the queue order and that M-109 shows "Evidence-based fallback" and "Fallback used Yes" with no alert, and requires a clean console and network.
  - The 390 px long-text probe sets every explanation field and the action text to its maximum length (with unbroken tokens) and checks there is no overflow and the action stays visible.

## Real Dataset Result (all modes)

| Priority | Meter | Type | Severity | Confidence |
| --- | --- | --- | --- | --- |
| 1 | M-109 | REAL_ANOMALY | HIGH | 0.9333333333333335 |
| 2 | M-112 | DATA_QUALITY | HIGH | 0.9172951138412555 |
| 3 | M-104 | EXPLAINABLE_ANOMALY | MEDIUM | 0.7284010583598136 |
| 4 | M-106 | FALSE_POSITIVE | LOW | 0.7563607765495244 |

These are identical in deterministic mode (black-box and E2E), in forced-fallback mode (unit/integration outcome equality, black-box, E2E) and in the live Ollama smoke. The engine and its configuration were not changed.

## Live Ollama Validation (optional, manual)

| Item | Value |
| --- | --- |
| Ollama | 0.34.4 (CUDA on GTX 1050, 4 GiB) |
| Model | `llama3.2:3b`, 2.0 GB, Q4_K_M, 3.2B parameters; installed with `ollama pull` (digest verified by Ollama). Chosen because no model was installed and it is a stable general instruct model that fits the GPU; not a product requirement |
| Procedure | Disposable PostgreSQL 18.6 → migrate/seed → compiled API with `EXPLANATION_PROVIDER=ollama`, `OLLAMA_MODEL=llama3.2:3b`, `OLLAMA_TIMEOUT=120s` → login → analysis → API reads. UI presentation was validated separately through the production-build Playwright and manual investigation checks; no live-model test is required by the automated suite |
| Result (final audited prompt/payload/validator) | COMPLETED in 28.047 s; 4/4 `OLLAMA`, `fallback_used = false`, model and `energy-explanation-v1` persisted; the analytical findings and order were unchanged |
| Latency | M-109 6.501 s; M-112 7.591 s; M-104 7.263 s; M-106 6.436 s |

**Manual review of the four narratives (final run):**

| Meter | Review |
| --- | --- |
| M-109 | "Persistent energy consumption increase sustained over time." No root cause or explanatory event was invented; controlled evidence and action remained canonical |
| M-112 | "The energy meter's electrical readings are inconsistent with each other, indicating unreliable data." No billing, equipment-failure, damage, revenue or future-failure claim; controlled evidence and action remained canonical |
| M-104 | The operational-change context explains the deviation without a forecast, billing, efficiency or cost consequence; controlled validation action remained canonical |
| M-106 | The scheduled outage and recovery support de-escalation without an invented repair, saving or success claim; controlled monitoring action remained canonical |

**Independent-audit remediation:**

1. The prior numeric allow-list could accept a correct number attached to the wrong metric. The model payload is now qualitative and numeric-free, and any model-authored number is rejected. Tests cover `110%`, `22%`, `111h`, `2%` and `1.7%` reassignment attempts.
2. The prior prompt alone did not reliably prevent unsupported consequences: M-112 mentioned billing. `why_it_matters` is now schema-constrained and independently validated against the canonical classification meaning (optionally plus one controlled severity sentence), so that billing remark and generic consequences cannot persist.
3. The generated provider had been able to rewrite evidence and broaden action wording. Those fields are now schema-constrained and independently validated against deterministic controlled text. Only the concise qualitative summary is freely authored.
4. Strict parsing now rejects duplicate fields and trailing content; adapter tests cover exact 64 KiB response boundaries and redirect refusal. Parent-run cancellation stops explanation work without invoking or logging fallback.

No analytical rule or engine code changed.

## Security, Data And Prompt Review

- **Payload**: a test asserts it contains no password, cookie, session, database, signing or authorization data and no per-reading signals. Only one finding's evidence is sent.
- **Provider URL**: `OLLAMA_BASE_URL` is server configuration (http(s), credentials rejected). No request can choose a provider URL, so there is no SSRF surface. Redirects are not followed.
- **Rendering and logs**: generated text is rendered as text (tested with `<script>`); prompts, responses and event descriptions are not logged (black-box assertion); raw provider errors are neither logged nor exposed.
- **Secrets**: the diff scan found none. `.env.example` has placeholders only. No hosted AI provider or API key was added.

## Phase Exit Gate

| Check | Result |
| --- | --- |
| Scope (boundary, two providers, config, prompt, parsing, grounding, fallback, persistence, provenance, API, UI, tests, docs) | Done |
| Deterministic default works with no model | Verified (all suites, E2E) |
| Fallback never fails a valid run; analytics failure still fails | Verified (integration, black-box, E2E) |
| Analytics unchanged | Verified (identical outcomes in every mode) |
| Regression Phases 01–04 | All previous tests pass; expectations for the new stage (step count, migration list) were extended, not weakened |
| Race | Clean on Linux |
| Visual 1440/768/390 | Explanation panel reviewed on real screenshots; maximum-length probe at 390 px |
| Docs and traceability | explainability, prompt contract, architecture, ADR-006 (implementation), data model, OpenAPI, README (two modes), decisions (TD-30, OD-16), testing strategy, design system §11, traceability |
| Commit | Not created (Phase 05 left uncommitted) |

## Independent Audit Result

**COMPLETE WITH NON-BLOCKING NOTES.** The audit reproduced the full unit, PostgreSQL integration, real-stack frontend, fallback, static, shuffled and race suites; reviewed every Phase 05 path; and reran the live model after the prompt, payload and validator changes. The model cannot alter analytical authority, cannot author numeric evidence, consequences, evidence wording or action wording, and provider failure cannot fail a valid analysis.

## Known Limitations

- The freely authored qualitative summary remains generated language rather than a general semantic proof. It has no numbers and cannot replace the canonical controlled evidence or action shown beside it; each run persists it with provenance.
- Generated summary wording varies between runs; each run's text is persisted with its provenance.
- Live Ollama quality and latency depend on the local model and hardware; only `llama3.2:3b` on the development GPU was validated.
- There is no regeneration control, external knowledge, RAG or second-model judge; these are intentionally outside the challenge scope.
- Explanation fallbacks are logged but not yet counted as metrics (Prometheus is Phase 06).

## Next Phase

None authorized. Phase 06 was not started and requires explicit user authorization.

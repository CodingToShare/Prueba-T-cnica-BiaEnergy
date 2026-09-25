# ADR-006: Generative AI As An Optional Explanation Layer

- Status: Accepted
- Date: 2026-09-24

## Context

The challenge rewards explanations and recommendations supported by evidence. Generative models write fluent operator-facing text, but they are non-deterministic, may be unavailable, and can invent facts. The evaluator must be able to run the product without any model.

## Decision

- Introduce an `ExplanationProvider` boundary that turns an already-computed finding plus its structured evidence into operator-facing text (summary, explanation, recommended-action wording).
- `DeterministicExplanationProvider` builds text from templates over evidence. It is the default and the fallback.
- `OllamaExplanationProvider` (optional, local, open-source models) may rewrite the same evidence into more natural language.
- The LLM is **not authoritative** for anomaly existence, type, severity, numeric confidence, evidence, or baselines. It receives only structured evidence and must not add numbers, events, or causes not present in it.
- If the provider is not configured, times out, errors, or its output fails validation, the deterministic text is used and the response records which provider produced it.
- No prompts or model calls are implemented before Phase 05.

## Consequences

- The product is complete and testable offline; generative text is an enhancement.
- Two text paths must stay consistent with the same evidence; grounding checks are needed for the generative path.
- Latency of local models must not block analysis completion; generation is bounded and failure-tolerant.

## Alternatives Considered

- **LLM as detector/classifier**: rejected (ADR-004).
- **Hosted proprietary API as the only provider**: requires secrets and network access during evaluation.
- **No generative layer**: acceptable functionally, but forgoes a clear, safely bounded demonstration of generative AI.

## Implementation (Phase 05, 2026-09-25)

The decision is implemented without change of scope; these points make it concrete:

- **Boundary.** `analysisrun.ExplanationProvider` is declared by the run orchestration (its consumer) and implemented in `internal/explanation` by `Deterministic` and `Ollama`. Providers return four text fields plus source, model and prompt version. Nothing they return reaches the type, severity, confidence, priority, evidence or action code.
- **Generation at analysis time, persisted.** Explanations are produced once per finding in the run's `GENERATING_EXPLANATIONS` stage (sequential) and stored with the findings and their provenance in the same transaction. Reading a finding never calls a provider; there is no regeneration endpoint.
- **Failure isolation.** An unreachable, failing, slow, malformed or ungrounded generative reply is replaced by the deterministic text (`fallback_used`, sanitized code), and the run still completes. With Ollama the stage has its own time budget, so a slow model cannot turn a valid analysis into a timeout. Analytical, load and persistence failures still fail the run.
- **Validation.** The output is strict JSON with unique keys, length limits and no markup. The model receives a qualitative projection with no numeric facts or non-supporting variables; model-authored numbers are rejected. JSON-schema enums and validation keep `why_it_matters`, the evidence narrative and action wording equal to controlled deterministic text. The model authors only the concise summary. Event types and roles are checked against the evidence. The prompt is versioned (`energy-explanation-v1`, `docs/ai/prompts/`).
- **Local model (OD-16).** The product and every automated test run without a model. The optional live validation used `llama3.2:3b` (2.0 GB, Q4_K_M, 3.2B parameters): a small, stable general instruct model that fits the development GPU's 4 GiB and supports structured output. It is a local validation choice, not a requirement; any Ollama model can be configured with `OLLAMA_MODEL`.

Details: `docs/ai/explainability.md`.

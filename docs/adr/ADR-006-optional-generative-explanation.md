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

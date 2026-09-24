# Phase 05 — AI Investigation / Explainability Integration

- Status: **Planned / Not Started**
- Authorization: requires explicit user authorization.

## Objective

Formalize the `ExplanationProvider` boundary, add an optional grounded Ollama provider, and complete the investigation experience so every finding clearly shows what happened, why it matters, the evidence, and what to do next.

## Prerequisites

- Phases 03 and 04 Complete.
- Phase Entry Gate: complete backend regression, frontend unit/component regression, Playwright critical smoke flow.

## Scope

- `ExplanationProvider` with `DeterministicExplanationProvider` (default) and `OllamaExplanationProvider` (optional, configuration-enabled).
- Grounding and output validation, bounded timeout, fallback, `explanation_source` provenance, fallback logging.
- Investigation page: evidence → explanation → recommendation, changed variables, baseline comparison chart, related events, severity and confidence components.
- Resolve OD-16.

## Non-Goals

LLM-based detection, classification, severity, confidence, or evidence; RAG; vector stores; agent frameworks.

## Expected Validation

- Deterministic explanation tests per classification.
- Evidence grounding and hallucination-prevention tests (invented numbers/events/causes rejected).
- Provider contract tests; fallback on timeout, error, invalid output; LLM-unavailable behavior.
- Ollama adapter tests against a fake HTTP server; optional recorded live validation.
- Investigation page/component tests.
- Playwright investigation flow: evidence → explanation → recommendation, with no LLM configured.
- Visual validation at ~1440/768/390 px.
- Regression: complete previous suite.

## Evidence Required

Entry Gate result; commands and counts; product verified with no LLM; optional live Ollama notes; Exit Gate checks; traceability rows FR-AI-002, FR-INV-001, BR-07, BR-08 updated.

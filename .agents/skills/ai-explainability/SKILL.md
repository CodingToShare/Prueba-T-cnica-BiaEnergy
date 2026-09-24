---
name: ai-explainability
description: Implement explanation providers, evidence presentation, grounding validation, and fallback behavior for operator-facing explanations and recommendations.
---

# AI Explainability

## Use When

Changing `ExplanationProvider` implementations, explanation templates, generative integration (Ollama), evidence payloads, or the investigation experience.

## Responsibilities

Turn structured evidence into clear, grounded explanations answering what happened, why it matters, what evidence supports it, and what to do next (ADR-006, `docs/ai/explainability.md`).

## Required Rules

- The deterministic provider is the default and fallback and must always work.
- Generative providers receive only structured evidence and may only reword it.
- Validate generative output (sections, length, numbers and events against evidence); on failure, timeout, or error, fall back and record `explanation_source`.
- Bound generation time; never block or fail a run because of the LLM.
- Log and count fallbacks; never log secrets.
- Keep prompts versioned in code, small, and reviewed like code.

## Prohibited

LLM-decided existence, type, severity, confidence, evidence, baselines, priority, or action code; RAG, vector stores, agent frameworks; hosted APIs requiring secrets for the default path; ungrounded speculation presented as fact.

## Completion Checklist

- Template tests per classification pass.
- Grounding validation and fallback tests pass.
- Product verified with no LLM configured.

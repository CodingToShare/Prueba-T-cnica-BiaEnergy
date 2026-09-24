---
name: documentation
description: Keep product, architecture, AI, ADR, phase, traceability, and README documents accurate, categorized, concise, and aligned with verified evidence.
---

# Documentation

## Use When

Behavior, contracts, schema, configuration, architecture, analytics calibration, test evidence, or phase status changes; or during documentation audits.

## Responsibilities

Update only the owning document and link elsewhere. Owners: requirements/rules/decisions/scope/traceability → `docs/product/`; architecture → `docs/architecture/`; decisions → `docs/adr/`; analytics and explainability → `docs/ai/`; phases → `docs/phases/`; run instructions → `README.md`.

## Required Rules

- Keep categories distinct: REQUIREMENT, BUSINESS RULE, TECHNICAL DECISION, ASSUMPTION, OPEN DECISION, OUT OF SCOPE.
- Resolving an open decision records the decision, rationale, and date, and updates dependent documents.
- Planned work is never described as implemented; commands are documented only after they work.
- Calibrated thresholds are documented with rationale in `docs/ai/anomaly-analysis.md`.
- Product docs are tool-neutral: no attribution to coding assistants.
- Write concisely; every section answers a real engineering question.

## Prohibited

Duplicating the same paragraphs across documents; fabricated screenshots or results; marking evidence PASS for commands not executed after the final change; secrets or machine-specific paths.

## Completion Checklist

- Owning documents match the final diff.
- Traceability and phase evidence updated.
- No contradictions between `AGENTS.md`, skills, ADRs, architecture, and README.

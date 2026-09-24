---
name: code-review
description: Review an actual diff for correctness, analytics integrity, boundaries, security, data access, UX states, and test gaps before declaring any change or phase complete.
---

# Code Review

## Use When

Before declaring a change, phase, or submission complete, and whenever a review is requested. Always review the real diff, not the intended one.

## Responsibilities

Report concrete findings ordered by severity with file locations. Verify behavior against requirements, rules, ADRs, and the Definition of Done.

## Required Rules

- **Analytics integrity:** no branching on meter IDs, timestamps, or dataset literals; no reference to `expected_results.csv`; thresholds only in the engine configuration; the LLM never sets type/severity/confidence/evidence.
- **Boundaries:** thin handlers, pure engine, no analytics in the frontend, interfaces only at real boundaries.
- **Data access:** SQL-side filtering/sorting, no N+1, parameterized queries (sqlc or explicit pgx), timezone-naive source times vs `timestamptz` system instants (ADR-008), indexes matching queries.
- **Errors and security:** standard error body, no leaked internals, no secrets, validated input, safe logs.
- **Frontend:** loading/empty/error/retry states, accessibility, responsiveness, design tokens.
- **Tests:** changed risk is covered at the right level (unit/integration/functional); focused and regression commands actually run and pass; nothing skipped or weakened silently.
- **Gates:** Phase Entry Gate result recorded; design validation against the HTML reference done for UI changes.
- **Docs:** categories preserved; phase status and traceability reflect evidence only; product docs tool-neutral.

## Prohibited

Approving from file appearance; burying correctness issues under style comments; accepting dead code, duplicated rules, or speculative abstractions.

## Completion Checklist

- Findings actionable and evidence-based; blocking ones resolved.
- Relevant builds/tests executed and reported.
- No next phase started; no completion claimed without evidence.

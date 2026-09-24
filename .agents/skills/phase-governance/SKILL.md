---
name: phase-governance
description: Start, execute, audit, or close a roadmap phase through the Phase Entry Gate and Phase Exit Gate, enforcing explicit authorization, scope, regression safety, and evidence-based completion.
---

# Phase Governance

## Use When

Planning, starting, executing, auditing, or closing a phase in `docs/phases/roadmap.md`, or changing any phase status.

## Responsibilities

Ensure only the explicitly authorized phase is executed, on top of a verified baseline, within its scope, and closed only with objective evidence.

## Phase Entry Gate (before any new phase work)

1. Confirm the user explicitly authorized this phase and the previous required phase is Complete.
2. Read `AGENTS.md`, `.agents/context/project-context.md`, the active phase document, relevant product requirements, applicable ADRs, relevant skills, and the Definition of Done.
3. Inspect `git status`; identify unrelated or uncommitted existing changes and preserve them.
4. Run the baseline validation suite applicable at this point (roadmap "Baseline Validation" table), using the concrete commands recorded by earlier phases.
5. Confirm previously accepted builds and tests still pass.
6. Validate the previous phase's recorded evidence.
7. Review its known limitations and deferred work.
8. Confirm the requested work belongs to the authorized phase.
9. Record the gate result in the active phase document.

If the baseline is broken: do not continue silently. Report the phase as **Blocked by baseline regression** until understood. If it is a regression of accepted work, repair it first, without unrelated scope expansion. Never hide a failing test.

## Execution Rules

- Stay inside scope and non-goals; resolve only the open decisions assigned to the phase and record them.
- Add or update tests together with the implementation (Implement → Validate → Review → Document → Complete).
- Surface blockers instead of inventing behavior.

## Phase Exit Gate

Run every **applicable** check and record results: build; formatting; linting; static analysis; TypeScript type checking; unit, integration, and functional/E2E tests; database validation; security and secret review; actual git diff review (`code-review` skill); full applicable regression; manual acceptance; design validation for UI; documentation; traceability; known limitations; visible evidence. Mark a check N/A only with a meaningful reason. An applicable level is never skipped for time without recording the limitation. A phase with a known blocking failure is not Complete.

## Prohibited

Starting the next phase automatically; inferring authorization; marking Complete from file existence; skipping the Entry Gate; changing another phase's status without evidence; pulling later scope forward silently.

## Completion Checklist

- Entry Gate and Exit Gate results recorded in the phase document.
- Roadmap status and traceability updated.
- Work stopped; the next phase awaits explicit authorization.

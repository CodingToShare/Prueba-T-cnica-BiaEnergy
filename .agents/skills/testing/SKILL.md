---
name: testing
description: Choose and write unit, integration, and functional tests for changed behavior, keep regression confidence, and record real evidence.
---

# Testing

## Use When

Any behavior changes; any bug is fixed; any data rule, API contract, analytics logic, or UI behavior changes; or regression risk changes.

## Responsibilities

- Choose the cheapest reliable test level for each behavior (`docs/testing/testing-strategy.md` §3).
- Maintain meaningful regression confidence across phases.
- Validate important real boundaries (PostgreSQL, HTTP, the full stack for journeys).
- Preserve determinism.

## Required Rules

- Unit tests for isolated deterministic behavior (analytics math and decisions, validation, state transitions, UI states).
- Integration tests with real PostgreSQL (testcontainers-go) wherever database or API+DB behavior matters.
- Functional/E2E tests with Playwright wherever a user flow matters, against the real stack.
- Analytics acceptance scenarios assert evidence-backed semantics; meter IDs appear only in tests.
- Descriptive scenario names stating condition and expected outcome.
- Deterministic execution: injected clock, fixed seeds, no arbitrary sleeps, no dependence on execution order.
- No hidden skipped tests; every skip is listed with a reason.
- No fake success evidence: report only commands actually run after the final change.
- The main suite never depends on a live LLM.

## Prohibited

Coverage theater; excessive mocking; testing private implementation details; duplicating identical assertion matrices across all layers; hiding or retrying away flakes; weakening assertions to get a green build; writing synthetic data into `data/input/`; using `expected_results.csv`.

## Completion Checklist

- Focused tests for the change pass.
- Previously applicable regression tests pass.
- Actual commands and results recorded in the active phase document.
- Traceability matrix updated with executed evidence.

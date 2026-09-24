# Definition Of Done

Applies to every change and every phase exit. A category may be marked **N/A only with a meaningful reason** recorded in the phase document. Existing files or code that "looks done" never constitute completion.

Sequence: Implement → Validate → Review → Document → Complete.

## 1. Phase Entry Validation
- [ ] Phase explicitly authorized; previous required phase Complete.
- [ ] Phase Entry Gate executed and recorded; baseline green or the phase reported "Blocked by baseline regression".

## 2. Build Verification
- [ ] Affected backend and frontend projects build from a clean state.

## 3. Formatting, Lint, Static Analysis
- [ ] Formatter clean (e.g., gofmt); linters and static analysis (e.g., go vet, ESLint) pass; TypeScript type check passes when frontend is affected.

## 4. Unit Tests
- [ ] New/changed deterministic behavior covered by meaningful unit tests; they pass.

## 5. Integration Tests
- [ ] Real boundaries touched (PostgreSQL, API + DB, ingestion) covered with real dependencies; they pass.

## 6. Functional / E2E Tests (when applicable)
- [ ] User-visible flows affected are covered by Playwright against the real stack; they pass.

## 7. Data / Database Verification (when applicable)
- [ ] Migrations apply from an empty database; constraints hold; dataset counts and integrity verified; source files unmodified.

## 8. UI / Design Validation (when applicable)
- [ ] Semantic tokens used; views reviewed at ~1440/768/390 px; compared with `docs/design/reference/design-system-v3.html`; loading/empty/error/disabled states present.

## 9. Accessibility Basics (when UI changes)
- [ ] Labels, keyboard operation, visible focus, AA contrast, color not the only signal.

## 10. Security And Secrets
- [ ] No secrets committed; configuration via environment; SQL parameterized; input validated; no internals leaked in errors or logs.

## 11. Observability (where applicable)
- [ ] Useful structured logs for new flows; health/readiness/metrics unaffected or extended.

## 12. Manual Acceptance
- [ ] A short reproducible scenario was executed and its observation recorded.

## 13. Documentation
- [ ] Owning documents updated without duplication; planned work not described as implemented; decisions recorded in the right category.

## 14. Traceability
- [ ] `docs/product/traceability-matrix.md` rows updated with executed evidence only.

## 15. Code Review / Actual Diff Review
- [ ] The real diff reviewed with the `code-review` skill; findings resolved or recorded. No meter-ID branching, no evaluator-file use.

## 16. Regression
- [ ] Focused tests plus all previously applicable regression tests pass; no test hidden, skipped silently, or weakened.

## 17. Known Limitations And Deferred Work
- [ ] Limitations, skipped levels (with reason), and deferred items recorded with the phase that owns them.

## 18. Phase Evidence
- [ ] Phase document records Entry Gate result, exact commands, counts, outcomes, and manual/design observations.

## 19. Reproducibility
- [ ] Another engineer can reproduce the result from documented steps; commands documented only once they work.

## 20. No False Claim Of Completion
- [ ] Status reflects evidence; no known blocking failure; the next phase was not started.

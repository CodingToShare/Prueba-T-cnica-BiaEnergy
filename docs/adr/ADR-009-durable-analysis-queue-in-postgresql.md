# ADR-009: PostgreSQL-Backed Analysis Queue And Truthful Run Stages

- Status: Accepted
- Date: 2026-09-24
- Amends: ADR-007 (stage list, startup recovery rule, concurrency policy OD-14)

## Context

ADR-007 accepted background execution of analysis runs by an in-process goroutine, a fine-grained stage list (`READING_DATA`, `BUILDING_BASELINES`, `DETECTING_ANOMALIES`, `CORRELATING_EVENTS`, `CLASSIFYING`, `GENERATING_EXPLANATIONS`) and marking every non-terminal run `FAILED` at startup. Implementing Phase 03 showed three problems:

- The Phase 02 engine computes baselines, detection, correlation and classification in one call that takes milliseconds. Reporting those as separate stages would require simulated progress. Explanations are Phase 05 and do not exist yet.
- A run queued just before a restart has done no work. Failing it loses a valid request; the work can simply be picked up from PostgreSQL.
- OD-14 (concurrent runs) needs a guarantee that holds when two requests race, which an application-level "check, then insert" cannot give.

## Decision

- **PostgreSQL is the source of truth for pending work.** `POST /api/v1/ai/analyze` inserts a `QUEUED` row and answers 202. One in-process worker claims the oldest `QUEUED` run atomically (`UPDATE … WHERE id = (SELECT … FOR UPDATE SKIP LOCKED)`) and executes it. An in-memory signal wakes the worker immediately; a 10-second poll picks up anything the signal missed, such as runs queued before a restart. There is no broker and no generic job framework.
- **One active run at a time (OD-14).** A partial unique index (`analysis_runs_single_active`) allows at most one `QUEUED` or `RUNNING` run. A request that finds an active run gets that run back (202, `created: false`) instead of a duplicate. The database enforces this under concurrency.
- **Truthful coarse stages.** Status `QUEUED → RUNNING → COMPLETED | FAILED`. The stage within a run is `QUEUED → LOADING_DATA → ANALYZING → PERSISTING_RESULTS → COMPLETED` (or `FAILED`), with progress 0, 10, 35, 85 and 100 persisted at each real transition. A failed run keeps the progress of the stage that failed. There are no artificial delays.
- **Atomic results.** Meter statuses, findings and the transition to `COMPLETED` are written in one transaction. Any error rolls everything back and records `FAILED` with a safe code (`load_failed`, `analysis_failed`, `persistence_failed`, `timeout`, `interrupted`) and a fixed message, using a separate short operation that still runs after cancellation.
- **Startup recovery.** A run left `RUNNING` by a stopped process is marked `FAILED` (`interrupted`). `QUEUED` runs are kept and processed. There is no automatic retry.
- **Bounds and shutdown.** Each run has a 2-minute timeout (the supplied dataset takes milliseconds). On shutdown the HTTP server stops accepting requests first; then the worker context is cancelled and awaited. An interrupted run is recorded as `FAILED` (`interrupted`); if even that write fails, startup recovery marks it.
- **Current state.** Every read of "current" analytical state uses the latest `COMPLETED` run. Runs are never deleted or overwritten, so history is kept.
- **Execution provenance (TD-23 clarification).** The atomic claim records the executing service's engine version and configuration. A queued request surviving a deployment uses the deployed policy; its request-time metadata is refreshed before execution. Completed-run metadata remains unchanged.
- **Source snapshot.** Readings and events load in one read-only REPEATABLE READ transaction, closed before computation. Result persistence uses a separate write transaction.

## Consequences

- Progress is honest. A fast run often shows `COMPLETED` on the first poll, which is correct.
- A queued request survives a restart; an interrupted one is visibly `FAILED`, never stuck `RUNNING`.
- Startup recovery assumes a **single API process**: a second process would mark the first one's running run as interrupted. Running several API processes requires a heartbeat or lease column first, and a new decision.
- Atomic claims tolerate competing claimers, but the application is not safe for multiple API processes: startup recovery requires the single-process assumption above.

## Alternatives Considered

- **ADR-007's fine-grained stages**: they would describe work the engine does not perform separately, so progress would have to be simulated.
- **In-memory queue only**: loses queued runs on restart and cannot enforce one active run across processes.
- **Returning 409 for a second request while one is active**: equally safe, but it makes the client handle an error for a normal double click. Returning the active run keeps one simple polling flow.
- **Message broker or job framework**: out of scope (`docs/product/out-of-scope.md`) with no demonstrated need.

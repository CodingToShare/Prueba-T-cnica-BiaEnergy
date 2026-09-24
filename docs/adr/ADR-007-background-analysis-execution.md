# ADR-007: Lightweight Background Analysis Execution

- Status: Accepted
- Date: 2026-09-24

## Context

"Run AI Analysis" must show process state (§13). Analysis over 4,032 readings is fast, but explanation generation may be slower, and the UI must be able to poll progress and survive a page reload. There is no requirement for horizontal scaling or cross-process work distribution.

## Decision

- `POST /api/v1/ai/analyze` persists an `AnalysisRun` (state `QUEUED`) and returns its id immediately (HTTP 202).
- A lightweight in-process goroutine executes the run and persists progress through the states `QUEUED → READING_DATA → BUILDING_BASELINES → DETECTING_ANOMALIES → CORRELATING_EVENTS → CLASSIFYING → GENERATING_EXPLANATIONS → COMPLETED`, or `FAILED` with a safe error summary.
- `GET /api/v1/ai/analysis/{id}` returns the persisted state, progress, and summary; the frontend polls it with TanStack Query.
- Results are written transactionally so a run's anomalies become visible only when the run completes.
- On startup, runs left in a non-terminal state by a crash are marked `FAILED`.
- The concurrency policy (e.g., one active run at a time) is OD-14.

## Consequences

- No broker or worker infrastructure; the whole flow is observable in PostgreSQL.
- Work in progress is lost if the process dies (acceptable for the MVP; handled by the startup recovery rule).
- Runs are bounded by a context with timeout and cancelled on graceful shutdown.

## Evolution Path (Not Implemented)

API → durable queue/message broker → independent analysis workers, reusing the pure engine package unchanged. Adopt only when volume or isolation needs demonstrate it.

## Alternatives Considered

- **Synchronous request**: simplest, but cannot show staged progress and couples UI timeouts to generation latency.
- **Kafka/RabbitMQ/NATS workers**: operational cost with no current requirement.
- **Server-sent events/WebSockets for progress**: nicer latency, but polling is sufficient and simpler to test.

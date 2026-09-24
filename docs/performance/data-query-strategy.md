# Data Query Strategy

Guidance for data access as the implementation grows. The dataset is small (4,032 readings), so correctness and clean query shapes matter more than tuning. Measure before optimizing.

## Expected Query Shapes

| Use | Shape |
| --- | --- |
| Meter detail history | Readings for one meter within a time range, ordered by time |
| Analysis input | All readings and events, streamed per meter or loaded once per run |
| Meters list | One row per meter with aggregates (consumption window, variation, computed status, top severity), filtered/searched/sorted server-side |
| Anomalies list | Findings of the current run ordered by priority, filterable by type/severity |
| Dashboard | A small set of aggregates over meters, readings, and the latest run |

## Rules

- Filtering, sorting, searching, and aggregation run in SQL, not in Go or the browser.
- Avoid N+1: list endpoints fetch aggregates in one query (joins, lateral joins, or pre-aggregated CTEs), not one query per meter.
- Select only needed columns; no `SELECT *` in application queries.
- Paginate lists that can grow (anomalies, readings ranges); the 12-meter list may stay unpaginated until volume says otherwise.
- Time ranges are half-open (`>= start AND < end`) on the timezone-naive source timestamps (ADR-008).
- Indexes are driven by real queries: `(meter_id, timestamp)` on readings from the start; add others (e.g., anomalies by run and priority) when a query needs them, verified with `EXPLAIN`.
- Store derived analysis results rather than recomputing them per request.

## Deferred Until Volume Demonstrates Need

Time-based partitioning of readings, materialized views for dashboard aggregates, time-series extensions, caching layers. Each needs measurements and, if architectural, an ADR.

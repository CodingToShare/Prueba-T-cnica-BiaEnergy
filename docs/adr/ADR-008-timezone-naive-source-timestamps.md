# ADR-008: Timezone-Naive Source Timestamps

- Status: Accepted
- Date: 2026-09-24
- Amends: ADR-003 (timestamp clause) for source observations

## Context

ADR-003 stated that timestamps use `timestamptz`, and assumption AS-02 proposed interpreting the supplied timestamps as UTC. The supplied `readings.csv` and `events.csv` contain wall-clock times with no offset (`2026-09-01 00:00:00`, `2026-09-11 00:00`). Storing them as `timestamptz` requires choosing a zone: UTC or the server/session zone. The challenge provides neither, so that choice would add information that does not exist. It could also shift hour-of-day values, which the hour-aware baselines depend on (ADR-004).

## Decision

- Source observation times are stored as PostgreSQL `timestamp without time zone`: `readings.reading_timestamp` and `events.event_timestamp`. They keep the source wall clock exactly.
- In Go, these values are `time.Time` with location UTC used only as a neutral carrier. No conversion is ever applied. The only place that applies source timestamp semantics is `ingestion.ParseSourceTimestamp`.
- Timestamps the system creates itself (for example future analysis run start and finish times) use `timestamptz`. ADR-003's clause stays valid for them.
- If a future source provides a zone or offset, the ingestion contract gains explicit source-zone metadata; a new ADR records that change.

## Consequences

- No invented zone; hour-of-day and day boundaries match the source exactly.
- Comparing source times with system instants (`now()`) needs care. Analytics compares source times only with other source times.
- API responses must present source times as local, zone-less values (for example `2026-09-12T14:00:00` without `Z`). This is defined in Phase 03.

## Alternatives Considered

- **`timestamptz` interpreted as UTC (former AS-02):** convenient, but asserts a zone the data does not state.
- **`timestamptz` in the server's zone:** results would depend on machine configuration.
- **Text columns:** lose type safety, ordering and range queries.

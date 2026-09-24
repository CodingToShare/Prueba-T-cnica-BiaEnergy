---
name: data-ingestion
description: Parse, validate, and load the source CSVs from data/input into PostgreSQL idempotently, reporting data-quality facts without altering source files.
---

# Data Ingestion

## Use When

Changing `cmd/seed`, CSV parsing, validation, or loading, or when source files change.

## Responsibilities

Load `readings.csv` and `events.csv` exactly and reproducibly, and surface data facts that analytics depends on.

## Required Rules

- Treat `data/input/` as read-only; never modify, reformat, or regenerate source files.
- Validate headers, types, ranges, and timestamp formats (readings and events use different formats); detect duplicates and gaps; report counts and distributions.
- Store CSV `status` as `source_status`; never interpret it as analytical health.
- Interpret timestamps per AS-02 until resolved; keep the rule in one place.
- Loads are idempotent (natural keys + upsert or truncate-and-load within a transaction) and fail loudly on malformed required fields.
- Stream rows; keep memory bounded even though the dataset is small.

## Prohibited

Creating synthetic replacement data; using or referencing `expected_results.csv`; silently dropping rows; parsing event descriptions into facts without an accepted decision (OD-09).

## Completion Checklist

- Integration test loads exactly 4,032 readings for 12 meters and all events; reload is a no-op.
- Validation report recorded in the phase document.

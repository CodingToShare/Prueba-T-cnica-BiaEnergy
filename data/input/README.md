# Source Dataset

The challenge input files, exactly as supplied. `cmd/seed` loads them into PostgreSQL (see `docs/architecture/data-model.md`).

| File | Content | SHA-256 |
| --- | --- | --- |
| `readings.csv` | 4,032 hourly readings, 12 meters (M-101…M-112), 2026-09-01 00:00:00 → 2026-09-14 23:00:00 | `01c953c2daa6503f9696a46096f34c635d67b7394054ad3f628a0230393b162f` |
| `events.csv` | 4 known events | `750d42f11c2c409af7094153fd07071d3aa3b5a8d68bb418c9fcaf7cbb0c9932` |

Both files are UTF-8 without BOM and use LF line endings. They were copied byte-for-byte from the files supplied with the challenge; the hashes were verified identical before and after the copy (Phase 01).

Rules:

- Never edit, reformat, re-sort or regenerate these files. The dataset acceptance test verifies their hashes are unchanged after an import.
- Never generate synthetic replacements. Test fixtures live in `src/backend/internal/ingestion/testdata/`.
- `expected_results.csv` is reserved for the evaluator. It must never be placed in this repository, searched for, or referenced by code, tests or documentation beyond this prohibition.
- `.gitattributes` marks these CSVs as non-text so Git never normalizes their line endings.

Headers:

```text
readings.csv: meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status
events.csv:   meter_id,event_timestamp,event_type,description
```

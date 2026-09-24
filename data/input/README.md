# Source Dataset

This directory holds the challenge input files exactly as supplied. They are loaded into PostgreSQL by the ingestion step planned for Phase 01.

| File | Content | Status |
| --- | --- | --- |
| `readings.csv` | 4,032 hourly readings for 12 meters over 14 days | Must be placed here by a person |
| `events.csv` | Known operational/data events per meter | Must be placed here by a person |

Rules:

- Copy the original files byte-for-byte. Never edit, reformat, re-sort, or regenerate them.
- Never generate synthetic replacements for these files.
- `expected_results.csv` is reserved for the evaluator. It must never be placed in this repository, searched for, or referenced by code, tests, or documentation beyond this prohibition.
- `.gitattributes` marks these CSVs as non-text so line endings are never normalized.

Expected headers:

```text
readings.csv: meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status
events.csv:   meter_id,event_timestamp,event_type,description
```

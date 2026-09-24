# Functional Requirements

Every entry below is a `REQUIREMENT` traced to a section of the original challenge (§N). Acceptance criteria describe observable completion without adding unrequested behavior. Interpretations that the challenge leaves open are linked to `assumptions-and-decisions.md` (OD-/AS-) instead of being resolved here. Planned phases are in `traceability-matrix.md`.

## Dashboard

| ID | Requirement | Source | Acceptance criteria |
| --- | --- | --- | --- |
| FR-DASH-001 | Show fleet KPIs: meter count, total consumption for the period, AI anomalies detected, high-priority count, aggregated AI confidence, last analysis date/time and status. | §5 | All six KPIs render from backend data; before any analysis, anomaly KPIs show an explicit "not analyzed" state. Aggregation definitions: OD-06, OD-07. |
| FR-DASH-002 | The dashboard lets the user identify quickly what requires attention first. | §2, §24 | The highest-priority anomaly is visible and navigable from the dashboard. |

## Meter Management

| ID | Requirement | Source | Acceptance criteria |
| --- | --- | --- | --- |
| FR-MTR-001 | List meters with consumption, variation vs baseline, computed status (OK / Alert / Critical) and anomaly severity. | §6 | All meters listed with the four values; window definitions per OD-06. |
| FR-MTR-002 | Filter meters: all, normal, alerts, critical. | §6 | Each filter returns only matching meters; server-side. |
| FR-MTR-003 | Search by `meter_id`. | §6 | Matching meters returned. |
| FR-MTR-004 | Sort by consumption, variation, or severity. | §6 | Order is correct and stable in both directions. |
| FR-MTR-005 | Meter detail shows current consumption, baseline, variation, status, and history; ideally voltage, current, and power factor. | §7 | Detail view renders the values and a time-series history including electrical variables. Known events are marked on the history. |

## Anomaly Analysis

| ID | Requirement | Source | Acceptance criteria |
| --- | --- | --- | --- |
| FR-ANL-001 | A "Run AI Analysis" action runs the analysis and shows process state. | §13 | Triggering creates an analysis run whose progress is visible until completion or failure. |
| FR-ANL-002 | The analysis follows Readings → Baseline → Detection → Correlation → Events → Explanation → Recommendation. | §13 | Each stage is represented in run progress and in the result. |
| FR-ANL-003 | Completion shows a summary such as "4 anomalies detected · 2 require priority attention". | §13 | Counts come from the persisted run result. |
| FR-DET-001 | Detect spikes and abrupt changes. | §8 | Covered by engine tests. |
| FR-DET-002 | Detect persistent changes relative to baseline. | §8 | Covered by engine tests. |
| FR-DET-003 | Detect outliers. | §8 | Covered by engine tests. |
| FR-DET-004 | Detect abnormal hourly patterns. | §8 | Covered by engine tests. |
| FR-DET-005 | Detect data-quality problems. | §8 | Covered by engine tests. |
| FR-DET-006 | Detect anomalous relationships among consumption, voltage, current, and power factor. | §8 | Covered by engine tests. |
| FR-DET-007 | Recognize false positives when an operational event explains the change. | §8 | Covered by engine tests. |
| FR-AI-001 | Each finding exposes at least `meter_id`, `anomaly`, `type`, `severity`, `confidence`, `reason`, `recommended_action`. | §10 | API anomaly payload contains these fields; output is never only true/false. |
| FR-AI-002 | Explanations and recommendations are supported by the data. | §10, §18 | Every finding references structured evidence (see `docs/ai/explainability.md`). |

## Anomalies And Investigation

| ID | Requirement | Source | Acceptance criteria |
| --- | --- | --- | --- |
| FR-ANOM-001 | AI Anomalies screen lists meter, type, severity, confidence, and action, including findings classified as false positives. | §11 | All classified findings of the latest completed run are listed, ordered by priority. |
| FR-INV-001 | Investigation view shows: what the AI found, variables that changed, comparison against baseline, related events, severity and confidence, recommended action, supporting evidence. | §12 | All seven elements visible for a selected anomaly. |

## Platform, Data And Delivery

| ID | Requirement | Source | Acceptance criteria |
| --- | --- | --- | --- |
| FR-API-001 | Provide the minimum API surface (see below). | §15 | Endpoints exist and are described in OpenAPI 3. |
| FR-DATA-001 | Persist Meter, Reading, Event, and Anomaly with the suggested fields. | §16 | Schema contains the entities; extended by TD-06. |
| FR-DATA-002 | Load `readings.csv` and `events.csv`. | §19 | All 4,032 readings and all events load reproducibly and idempotently. |
| FR-DATA-003 | `expected_results.csv` is not available to the model or the end user. | §19 | No code, config, or UI references it. |
| FR-AUTH-001 | The demo flow starts with a Login step. | §21 | Unauthenticated users reach a login screen before product pages. Mechanism is intentionally simple (OD-12). |
| FR-UX-001 | The application feels like an Energy Management SaaS product, not a set of test screens; demo path `Login → Dashboard → M-109 → Run AI Analysis → Anomaly → Explanation → Action`. | §21 | Consistent navigation and design system; the demo path works end to end. |
| FR-TECH-001 | The backend uses Go. | §14 | Backend implemented in Go. |
| FR-DEL-001 | Deliver a Git repository, working frontend and backend, and a 5–10 minute demo. | §20 | Delivered at Phase 06. |
| FR-DEL-002 | The evaluator can reproduce the system without manually repairing state. | Project owner delivery requirement (Phase 00 governance) | A documented mechanism starts dependencies, creates/migrates the database, imports the provided data, starts backend and frontend, and preferably resets the demo dataset predictably. Entry point: OD-19. |

## Minimum API Surface (FR-API-001)

The challenge suggests the routes below; TD-03 places them under `/api/v1`.

```text
GET  /api/v1/meters
GET  /api/v1/meters/{meterId}
GET  /api/v1/meters/{meterId}/readings
GET  /api/v1/anomalies
GET  /api/v1/anomalies/{id}
POST /api/v1/ai/analyze
GET  /api/v1/ai/analysis/{id}
GET  /api/v1/dashboard/summary
```

Health, readiness, and metrics endpoints are technical decisions (TD-10), not challenge requirements. Query parameters, pagination, and payload shapes are defined in Phase 03.

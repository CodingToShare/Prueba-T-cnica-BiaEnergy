# Demo Guide

A 5–10 minute walkthrough of the product on the supplied data, followed by short answers to likely technical questions. Setup is in the [README](../../README.md#quick-start-one-command).

## Before The Demo (1 minute, off-screen)

```sh
docker compose down -v --remove-orphans     # fresh database, no previous analysis
docker compose up --build -d --wait         # returns when every service is healthy
```

Open http://localhost:3000. The browser window should be at least 1280 px wide; the narrow layout can be shown later by resizing.

## Walkthrough

| # | Time | Do | Say |
| --- | --- | --- | --- |
| 1 | 0:00 | Sign in as `demo` / `bia-demo-2026` | A deliberately simple demo login: a signed HttpOnly cookie behind a same-origin proxy, with no tokens in the browser. |
| 2 | 0:30 | Show the dashboard before analysis | The dashboard is honest: 12 meters, 4,032 readings and 155,250.85 kWh come from the data, and every analytical KPI says "Not analyzed yet" instead of a fake 0. |
| 3 | 1:00 | Click **Run AI Analysis** | The run is queued in PostgreSQL and executed by a background worker. The progress shows real stages only; the deterministic engine takes well under a second. |
| 4 | 1:30 | Show the completion summary and KPIs | 4 findings, 2 high priority, the aggregate confidence, and the investigation queue in priority order. |
| 5 | 2:00 | Open **M-109** (priority 1) | Real anomaly, high severity. Consumption rose about 110% above its hourly baseline for 58 h, and current and power factor changed with it. The UNKNOWN event at the onset is shown as *context only*: it does not explain the change. Recommended action: inspect the meter and installation. |
| 6 | 3:00 | Scroll through the evidence and chart | Every number comes from stored evidence: baseline vs observed energy, changed variables, confidence components (evidence strength, not a failure probability), and the chart with the episode and the event at its exact time. |
| 7 | 4:00 | Point at the **Explanation** panel | The explanation is generated once during the run and persisted with its provenance. By default it is deterministic, built from the evidence. A local model can reword it, but classification, severity and confidence come only from the engine; the panel says so. |
| 8 | 4:45 | Open **M-112** | Data quality, not an energy problem: voltage, current and power factor are inconsistent while consumption stays near its baseline, and a recorded data-quality event corroborates it. Action: validate the sensor, wiring and communication. |
| 9 | 5:30 | Open **M-106** | Event-aware false positive: consumption fell about 80% during a *scheduled outage*, then recovered. It is detected and explained, and not escalated. **M-104** is the explainable counterpart: a real rise explained by an operational change. |
| 10 | 6:15 | Mention optional Ollama | With `EXPLANATION_PROVIDER=ollama` and a local `llama3.2:3b`, the same evidence is reworded by a model. If the model is unavailable, slow or ungrounded, the deterministic text is used and the run still completes. |
| 11 | 7:00 | Show engineering evidence (optional) | `docs/phases/` (per-phase evidence and audits), the CI workflow, `GET http://localhost:8080/metrics`, and the Playwright real-stack suite (`pnpm test:e2e`). |

Wrap-up line: *deterministic, evidence-backed decisions; language models only for wording; one command to run and reset.*

## Technical Talking Points

| Question | Short answer |
| --- | --- |
| Why deterministic analytics instead of LLM detection? | Decisions must be reproducible, explainable and testable. The engine gives the same output for the same input, every threshold is documented, and the four scenarios are automated tests. An LLM is non-deterministic and can invent facts, so it is only a wording layer (ADR-004, ADR-006). |
| Why median/MAD? | Both are robust to the anomalies we look for: a few extreme readings don't move the baseline or the spread. A robust Z plus a relative-deviation gate avoids flagging meters whose normal spread is tiny ([anomaly analysis](../ai/anomaly-analysis.md)). |
| Why a historical same-hour baseline? | Consumption has a daily profile. Comparing 14:00 with previous 14:00s separates real changes from normal daily shape. Flagged hours are excluded, so an ongoing episode doesn't become its own baseline, and no future data is used. |
| Why a PostgreSQL worker instead of Kafka? | One process, one small dataset, one active analysis at a time. A durable queue row with `FOR UPDATE SKIP LOCKED` and a partial unique index gives atomic claims, restart recovery and "one active run" without another system (ADR-009). A broker is an evolution path, not a need. |
| Why sqlc rather than an ORM? | The SQL stays explicit and reviewable (filters, sorting and aggregation in the database), and sqlc generates type-safe Go from it. There is no hidden query building and no N+1 surprises. CI checks the generated code for drift. |
| Why persist `AnalysisRun`? | Runs are asynchronous and must survive restarts, report truthful progress and failures, and keep provenance: engine version, configuration and explanation settings. It is also what the UI polls. |
| Why "latest COMPLETED" semantics? | A running or failed run never replaces the current results, so the product always shows a consistent, complete analysis, even while a new one runs or after one fails. |
| Why a signed cookie instead of a JWT? | One demo user and one API. An HMAC-signed HttpOnly SameSite cookie is simpler and safer in the browser (no token in JavaScript), and the same-origin proxy avoids CORS. Real identity management is out of scope. |
| Why is Ollama optional? | Reviewers must be able to run everything without a 2 GB download or a GPU, and CI must be reproducible. The deterministic provider is complete product behavior; the model is an enhancement with a guaranteed fallback. |
| Why can't generated text change decisions? | By construction: the provider returns only four text fields. Classification, severity, confidence, priority, evidence and action code are persisted from the engine. Evidence goes to the model as JSON data, never as instructions, and the output is validated (strict schema, grounded numbers and events), falling back if rejected. |
| Why the Next.js same-origin proxy? | The browser talks to one origin, so the session cookie stays first-party, there is no CORS, and the backend address stays server-only. |
| Why PostgreSQL `NUMERIC`? | The source values are decimals. `NUMERIC` stores and sums them exactly; for example, the 155,250.85 kWh total matches the CSV to the cent. The engine converts to float64 only for statistics. |
| Why do source timestamps have no time zone? | The CSV has wall-clock hours without an offset. Storing them timezone-naive (and formatting them without `Date` in the UI) avoids inventing an offset. System events such as analysis times are real instants in UTC (ADR-008). |
| How is it observed? | JSON logs with request and run IDs, `/healthz`, `/readyz`, and `/metrics` with route-template labels, run outcomes and explanation fallbacks. No identifiers are labels, so cardinality stays bounded. |
| What would production add? | Real identity (OIDC), HTTPS and restricted operational endpoints, a managed database with backups, image publishing and deployment automation, and dashboards/alerts on the existing metrics. Kubernetes, Grafana and the like fit there, not in this MVP. |

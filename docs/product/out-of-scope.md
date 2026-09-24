# Out Of Scope

Items below are intentionally excluded from the MVP. Adding any of them requires a new accepted ADR that demonstrates a real requirement, not a keyword match or speculative scale.

## Architecture And Infrastructure

- Microservices, a separate API gateway, Kubernetes.
- Message brokers and streaming platforms (Kafka, RabbitMQ, NATS).
- Redis or other caches.
- Elasticsearch or other search engines.
- TimescaleDB or other time-series extensions without measured need.
- Event sourcing and full CQRS.
- Distributed tracing stacks and large observability platforms.
- Firebase or other platforms adopted only to match job-description keywords.

## AI And Analytics

- Using an LLM to detect, classify, score severity, or compute confidence.
- Vector databases and RAG.
- LangChain-style agent frameworks.
- A separate Python ML service.
- Model training pipelines.

## Product

- Complex authentication and authorization (OAuth/OIDC, RBAC, multi-user management).
- Multi-tenancy.
- Meter create/update/delete through the UI (meters come from the dataset).
- Real-time streaming ingestion; the dataset is a static batch.
- Alert delivery (email, SMS, push) and ticketing integrations.
- Editing or annotating anomalies beyond what the flow requires.

## Engineering

- Speculative generic repositories, interfaces with a single implementation and no boundary value, and framework-like abstractions.
- Coverage-percentage targets as a goal in themselves.

## Documented Evolution Paths (Not Implemented)

These are recorded in `docs/architecture/architecture.md` so reviewers can see how the design grows; none is built now: a durable queue with independent analysis workers, time-based partitioning of readings, streaming ingestion, and multi-site tenancy.

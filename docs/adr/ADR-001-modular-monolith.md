# ADR-001: Pragmatic Modular Monolith

- Status: Accepted
- Date: 2026-09-24

## Context

A three-day MVP must deliver ingestion, analytics, an API, and a product-quality UI over a 4,032-row dataset. Reviewers value clear boundaries and senior judgment, not distributed-system ceremony. There is one team, one deployable, and no independent scaling requirement.

## Decision

Build a pragmatic modular monolith: one Go API process (with in-process background analysis), one Next.js frontend, and one PostgreSQL database. Modules are Go packages organized by capability with explicit, narrow dependencies. The frontend talks only to the versioned HTTP API.

## Consequences

- One deployment, one transaction boundary, trivial local setup with Docker Compose.
- Boundaries are enforced by package design and review rather than network isolation; reviewers must watch for cross-module leakage.
- Evolution paths (separate analysis workers behind a queue, see ADR-007) remain possible because the analysis engine is a pure package with no HTTP or database dependency.

## Alternatives Considered

- **Microservices** (e.g., ingestion, analytics, API services): adds networking, deployment, observability, and consistency costs with no requirement driving them.
- **Serverless functions**: poor fit for a long-running analysis run with progress and for local evaluation.
- **Layered "clean architecture" projects ported from other ecosystems**: adds indirection that idiomatic Go does not need (see ADR-002).

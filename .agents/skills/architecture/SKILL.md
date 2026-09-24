---
name: architecture
description: Preserve modular-monolith boundaries and record significant decisions when adding modules, packages, libraries, infrastructure, or moving responsibilities.
---

# Architecture

## Use When

Creating or moving Go packages or frontend features, adding a dependency or infrastructure component, changing API strategy, persistence strategy, analysis execution, or AI responsibilities.

## Responsibilities

Keep the system a pragmatic modular monolith (ADR-001) whose boundaries match `docs/architecture/architecture.md` §3. Keep complexity proportional to current requirements.

## Required Rules

- Read `docs/architecture/architecture.md`, relevant ADRs, and `docs/product/out-of-scope.md` first.
- Name the concrete current problem before adding an abstraction, package, or library.
- The analysis engine stays pure (no DB/HTTP/clock/network). Handlers stay thin. The frontend holds no analytics rules.
- Interfaces are declared by consumers and only at meaningful boundaries.
- Significant decisions get an ADR (Status, Context, Decision, Consequences, Alternatives considered); superseding requires a new ADR.

## Prohibited

Anything in `out-of-scope.md` without an accepted ADR; generic repositories; interfaces for every struct; porting layered architectures from other ecosystems; letting the LLM own any authoritative value.

## Completion Checklist

- Boundaries intact; no cyclic or leaking dependencies.
- New technology justified and pinned; ADR added when significant.
- Architecture document updated if responsibilities moved.

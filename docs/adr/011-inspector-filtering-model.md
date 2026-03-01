# ADR-011: Inspector Timeline Filtering Model

## Status
Accepted

## Context
With history hydration and live stream enabled, the inspector timeline grows quickly.
Developers need fast narrowing controls to isolate command/event lifecycles without leaving the primary timeline view.

## Decision
- Add client-side filtering in inspector for:
  - free-text search (message name, source metadata, summary text)
  - envelope type (`lifecycle`, `event`, `command`)
  - source role (`daemon`, `dev-agent`, `inspector`)
- Apply filters on normalized timeline entries after merge/dedupe, not during socket ingestion.
- Keep filter state local to the session (no backend contract changes in this increment).

## Consequences
- Filtering stays responsive and avoids additional daemon query complexity.
- Timeline correctness remains centralized in merge logic while display concerns stay in UI state.
- Search semantics are intentionally broad; advanced query grammar can be added later without schema changes.

## Reversibility
High.
The filtering model is purely client-side and can be replaced or extended independently of protocol/storage layers.

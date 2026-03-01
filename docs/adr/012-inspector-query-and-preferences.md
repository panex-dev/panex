# ADR-012: Inspector Query Operators and Local Filter Persistence

## Status
Accepted

## Context
Basic filtering improved timeline navigation, but high-volume sessions still require faster narrowing patterns.
Developers also lose context when refreshing the inspector because filter state resets every load.

## Decision
- Add lightweight search operators to inspector query input:
  - `name:` for envelope names
  - `src:` for source role/id matching
  - `type:` for envelope type matching
- Keep query parsing intentionally small and token-based so it remains understandable and testable.
- Persist filter state (`search`, `messageType`, `sourceRole`) in browser localStorage with safe fallback behavior.
- Continue applying filtering only at the presentation layer after merge/dedupe.

## Consequences
- Operators make timeline triage significantly faster without backend changes.
- Inspectors reopen with prior filter context, reducing repetitive setup during iterative debugging.
- Query grammar is intentionally minimal; richer syntax can be layered later without changing transport contracts.

## Reversibility
High.
The feature is fully client-side and can evolve independently of daemon/event-store behavior.

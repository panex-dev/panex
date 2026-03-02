# ADR-010: Inspector Timeline Delivery Over WebSocket

## Status
Accepted

## Context
Panex now emits and persists protocol envelopes, but developers still need a visual surface that makes event flow inspectable in real time.
The first inspector milestone is a lightweight UI that can bootstrap with history and continue streaming live updates.

## Decision
- Build inspector as a dedicated SolidJS package under `inspector/`.
- Use the existing daemon WebSocket channel and protocol envelope format instead of adding a second API surface.
- On connect, send `hello`, wait for `hello.ack`, then issue `query.events` to hydrate timeline history.
- Merge `query.events.result` snapshots with live envelopes in one bounded in-memory timeline list.
- Keep rendering intentionally simple (event cards + metadata summary) until inspector interaction requirements are clearer.

## Consequences
- Inspector can be developed and shipped independently from daemon/agent runtime packaging.
- Timeline behavior remains protocol-driven and does not require direct SQLite access from the browser.
- UI state management stays predictable because both historical and live data normalize to one timeline entry model.

## Reversibility
High.
We can evolve rendering, filtering, and query ergonomics without changing the core handshake/query/live flow.

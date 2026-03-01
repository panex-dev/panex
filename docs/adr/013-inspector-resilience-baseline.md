# ADR-013: Inspector Resilience Baseline

## Status
Accepted

## Context
Inspector functionality depended on an always-on daemon connection.
Any transient WebSocket disconnect froze the timeline until a manual page refresh.

## Decision
- Add automatic reconnect with bounded exponential backoff in inspector runtime.
- Use `createMemo` for filtered timeline derivation to avoid repeated recomputation during rendering.
- Align initial `query.events` request limit with local timeline buffer size (`defaultTimelineLimit`).
- Wrap app rendering in a Solid `ErrorBoundary` fallback so unexpected render errors surface safely.

## Consequences
- Inspector self-recovers from daemon restarts and transient transport interruptions.
- Timeline rendering becomes cheaper and more predictable under high event volume.
- Query/bootstrap behavior is now consistent with local retention semantics.

## Reversibility
High.
All changes are client-side and can evolve independently of daemon protocol/storage contracts.

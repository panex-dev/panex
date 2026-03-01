# ADR-009: SQLite Event Persistence and query.events Retrieval

## Status
Accepted

## Context
Panex can now rebuild and issue reload commands, but we still lack durable protocol history.
Without persistence, inspector and debugging workflows lose all context on daemon restart.

## Decision
- Persist protocol envelopes in a local SQLite database using WAL mode.
- Store envelopes as MessagePack blobs with append-only row ids and millisecond timestamps.
- Expose `query.events` (command) and `query.events.result` (event) protocol messages so connected clients can request recent history without direct DB access.
- Return query results in chronological order even though database reads are optimized via descending id scans.

## Consequences
- Runtime protocol history survives daemon process restarts.
- Future inspector timeline and replay features can consume one source of truth.
- WebSocket server now owns one additional subsystem (event store lifecycle), which increases startup/shutdown responsibilities.

## Reversibility
Medium.
The API contract (`query.events` + `query.events.result`) can remain stable while storage internals change (schema evolution, pruning policy, alternate backend).

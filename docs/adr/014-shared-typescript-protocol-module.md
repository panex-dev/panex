# ADR-014: Shared TypeScript Protocol Module

- Date: 2026-03-01
- Status: Accepted

## Context

The Dev Agent and Inspector each maintained their own protocol definitions (`protocol.ts`).
That duplication made protocol evolution risky:

- New message names could be added in one client but omitted in the other.
- Runtime envelope validation could diverge.
- Type-level drift stayed invisible until integration testing.

At this stage the protocol is already central to the product loop (`hello`/`hello.ack`, `build.complete`, `command.reload`, `query.events`), so duplication now creates compounding maintenance cost.

## Decision

Create a single shared TypeScript protocol module at `shared/protocol/src/index.ts` and move both clients to consume it directly.

Scope of this ADR:

- Centralize protocol names, roles, and type mapping.
- Centralize envelope/message guards used by websocket handlers.
- Add a dedicated shared test suite to prevent map/name drift.
- Remove client-local `protocol.ts` files from `agent` and `inspector`.

## Consequences

Positive:

- One canonical TS contract for both clients.
- Protocol changes now require one edit for TS consumers.
- Runtime guard behavior stays consistent across clients.

Tradeoff:

- The Go protocol (`internal/protocol/types.go`) and TS protocol remain two source files; cross-language drift must still be reviewed intentionally.

Follow-up:

- Consider generating TS protocol constants from Go in a future hardening pass.

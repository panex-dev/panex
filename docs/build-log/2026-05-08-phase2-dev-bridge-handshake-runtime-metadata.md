# Phase 2 Dev Bridge daemon — handshake runtime metadata

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The daemon bridge already negotiated auth, sessions, and capabilities over `hello` / `hello.ack`, but the negotiated runtime identity stopped at the socket boundary. The daemon normalized `extension_id` during handshake, then dropped it from `hello.ack`, and the inspector Workbench only showed generic connection status instead of the live session metadata already known to the bridge.

## Approach

- Added optional `extension_id` to the shared `HelloAck` protocol payload in both Go and TypeScript so the daemon can echo the normalized runtime extension identity it negotiated during handshake.
- Updated the daemon handshake path to populate that `extension_id` in successful `hello.ack` responses while preserving the existing auth-failure behavior.
- Added a typed inspector-side `BridgeSession` model derived from `hello.ack`, stored it in connection state, and surfaced that live metadata in the Workbench connection card:
  - daemon version
  - negotiated session ID
  - negotiated extension ID when present
  - negotiated capabilities
- Extended agent diagnostics to include the echoed extension identity in `hello.ack` summaries so bridge logs capture the same negotiated runtime context.
- Added regression coverage across protocol parity, daemon handshake tests, integration tests, agent diagnostics, inspector connection normalization, and Workbench modeling.

## Risk and mitigation

- Risk: protocol field drift between Go and TypeScript could silently break message decoding.
- Mitigation: this slice uses the shared protocol parity suite plus targeted Go/TypeScript tests that exercise the new `HelloAck.extension_id` field from both ends.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/protocol ./internal/daemon`
- `pnpm install --frozen-lockfile`
- `pnpm --dir shared/protocol test`
- `pnpm --dir agent test`
- `pnpm --dir inspector test`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by widening runtime/session state beyond handshake metadata, only once the next consumer need is explicit. This PR stops at negotiated live session identity already available at handshake time; it does not introduce a broader session-query or bridge-state API.

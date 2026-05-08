# Phase 2 Dev Bridge daemon — centralize the websocket message size ceiling in protocol parity contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The shared TypeScript protocol already exported `MAX_WEBSOCKET_MESSAGE_BYTES`, and browser-side transport/tests consumed that limit as part of the websocket framing contract. The Go daemon still carried its own local `1 << 20` read-limit constant, which meant the bridge’s maximum-frame contract was split across layers and not covered by the existing Go↔TypeScript parity checks.

## Approach

- Added `MaxWebSocketMessageBytes` to Go protocol ownership in `internal/protocol/types.go`.
- Extended `internal/protocol/parity_test.go` so Go parses and compares the exported TypeScript `MAX_WEBSOCKET_MESSAGE_BYTES` constant directly.
- Switched the daemon websocket server to use the protocol-owned constant for `SetReadLimit`.
- Replaced the remaining raw oversized-frame test literal in `shared/chrome-sim/tests/transport.test.ts` with the shared protocol export.

## Risk and mitigation

- Risk: moving one more numeric limit into protocol ownership could accidentally imply that all websocket transport tuning belongs in the protocol layer.
- Mitigation: this slice centralizes only the stable cross-language frame-size ceiling that both the daemon and browser transport must agree on. Deadlines, ping cadence, and connection-lifecycle policy remain daemon-local.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run 'TestTypeScriptProtocolParity|TestMaxWebSocketMessageBytes' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/daemon -run 'TestWebSocketHandshakeSendsHelloAckAndTracksConnection' -count=1`
- `pnpm --dir shared/protocol test`
- `pnpm --dir shared/chrome-sim test`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining cross-language transport invariants are still duplicated as raw literals instead of protocol-owned constants with parity coverage.

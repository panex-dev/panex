# Phase 2 Dev Bridge daemon — centralize the default websocket path and browser URL in protocol parity contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The first-party bridge clients all relied on the same default daemon websocket endpoint, but that contract was still scattered as raw literals: `agent`, `inspector`, and `chrome-sim` each carried their own `ws://127.0.0.1:4317/ws` default, the shared URL normalizer carried its own `"/ws"` path rule, and the daemon router still registered the websocket handler with a separate path literal. That left the default browser endpoint vulnerable to silent drift even though the real daemon defaults already lived in Go config.

## Approach

- Added protocol-owned websocket path and default browser URL constants:
  - `internal/protocol/types.go`
  - `shared/protocol/src/index.ts`
- Extended Go↔TypeScript parity so the exported TypeScript path and URL must still match the Go config defaults plus protocol websocket path.
- Switched the daemon router, `agent`, `inspector`, `inspector` preview build, and `chrome-sim` transport defaults to consume the shared contract instead of package-local literals.
- Added shared protocol coverage for the published default daemon websocket contract.

## Risk and mitigation

- Risk: moving the default browser endpoint into shared protocol ownership could blur the line between stable client-facing transport contracts and daemon-local server configuration.
- Mitigation: this slice centralizes only the default loopback websocket path and default browser-facing URL that first-party clients must agree on. Bind-address parsing, auth defaults, reconnect behavior, and other operational policy remain package-local or config-local.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run 'TestTypeScriptProtocolParity|TestMaxWebSocketMessageBytes|TestDefaultDaemonWebSocketPath' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/daemon -run 'TestWebSocketHandshakeSendsHelloAckAndTracksConnection' -count=1`
- `pnpm --dir shared/protocol test`
- `pnpm --dir agent test`
- `pnpm --dir inspector test`
- `pnpm --dir shared/chrome-sim test`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining browser-facing transport invariants are still duplicated as package-local literals instead of shared protocol exports with parity coverage.

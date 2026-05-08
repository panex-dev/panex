# Phase 2 Dev Bridge daemon — client-kind capability scoping

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The daemon already negotiated capabilities per session, but it still treated every client kind as eligible for the full daemon capability catalog. A `dev-agent` or `chrome-sim` could therefore request capabilities outside the bridge surface that role actually uses, and the handshake would negotiate them as long as the daemon knew the message name. That kept capability negotiation narrower than global availability only when each client behaved perfectly, instead of enforcing the role contract at the handshake boundary.

## Approach

- Added daemon-side capability allowlists for the concrete first-party client kinds that already have bounded bridge surfaces: `dev-agent` and `chrome-sim`.
- Switched `hello` / `hello.ack` capability negotiation to intersect the requested capability set with the client-kind-specific allowlist rather than the full daemon capability catalog.
- Preserved a compatibility fallback for unknown client kinds by continuing to negotiate against the full daemon capability catalog, so older or external clients do not break just because they predate explicit role scoping.
- Added websocket regression coverage for:
  - `dev-agent` sessions only negotiating `command.reload`
  - `chrome-sim` sessions only negotiating the browser bridge capabilities they actually use
  - unknown client kinds still falling back to the global capability catalog
- Corrected existing websocket tests that were relying on helper behavior rather than explicitly choosing the intended client role for capability negotiation.

## Risk and mitigation

- Risk: a first-party bridge client could have been relying on an over-broad capability set that was never part of its intended role.
- Mitigation: the change is covered at the handshake boundary with explicit client-kind tests, and unknown client kinds keep the old global fallback so this slice narrows only the first-party roles we can verify.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/daemon -run 'TestWebSocketBroadcastSkipsSessionsWithoutNegotiatedCapability|TestWebSocketHandshakeScopes|TestWebSocketHandshakeUnknownClientKindFallsBackToGlobalCapabilities|TestWebSocketHandshakeNegotiatesCapabilities|TestWebSocketCapabilityEnforcementRejectsUnnegotiatedMessage' -count=1 -v`
- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking whether the remaining handshake metadata and capability negotiation rules should also distinguish unsupported combinations of `source.role` and `client_kind`, but only if a concrete first-party client path still depends on that tighter contract.

# Phase 2 Dev Bridge daemon — chrome-sim source role provenance

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The protocol still had no distinct source role for `chrome-sim`, so the browser bridge had to label its handshake and `chrome.api.call` traffic as `inspector`. That leaked the wrong provenance into persisted bridge history, inspector timeline filters, and any debugging that relied on `src.role`. The daemon also did not tie a session to the source role it negotiated during `hello`, so even known first-party clients could drift to a different role in later command envelopes.

## Approach

- Added a real `chrome-sim` source role to the shared Go and TypeScript protocol contracts.
- Updated the browser transport to use `src.role = "chrome-sim"` for both `hello` and outbound `chrome.api.call` envelopes.
- Tightened the daemon handshake contract so known first-party `client_kind` values must use the matching source role:
  - `dev-agent` -> `dev-agent`
  - `inspector` -> `inspector`
  - `chrome-sim` -> `chrome-sim`
- Recorded the negotiated source role in session metadata and rejected later client commands that tried to use a different role than the one established during handshake.
- Extended the inspector timeline source-role filter to include `chrome-sim`.
- Added protocol parity, chrome-sim transport, and daemon websocket regression coverage for the new role and the tightened provenance checks.

## Risk and mitigation

- Risk: tightening source-role provenance could reject an existing first-party client that was still sending the old mismatched role.
- Mitigation: the same PR updates the only first-party browser transport that needed the new role, keeps unknown `client_kind` values out of the strict handshake mapping, and adds handshake plus post-handshake drift tests.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/protocol -run 'TestTypeScriptProtocolParity|TestPayloadFieldShapeParity' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/daemon -run 'TestWebSocketHandshakeScopes|TestWebSocketHandshakeUnknownClientKindFallsBackToGlobalCapabilities|TestWebSocketHandshakeRejectsKnownClientKindWithUnexpectedSourceRole|TestWebSocketCapabilityEnforcementRejectsUnnegotiatedMessage|TestWebSocketCapabilityEnforcementRejectsMessageSourceRoleDrift' -count=1 -v`
- `pnpm install --frozen-lockfile`
- `pnpm --dir shared/chrome-sim test`
- `pnpm --dir inspector test`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining handshake/session metadata should be enforced as stable session provenance, but only where there is a concrete first-party consumer or policy boundary behind it.

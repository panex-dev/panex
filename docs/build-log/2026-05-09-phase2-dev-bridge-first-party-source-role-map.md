# Phase 2 Dev Bridge daemon — centralize first-party client kind to source role mapping in protocol contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The first-party Dev Bridge handshake already treated `client_kind` and `src.role` as coupled identity fields, but that mapping still lived in several places: daemon-side validation, daemon test helpers, and the browser clients that build `hello` envelopes. That duplication made the contract easy to restate inconsistently even after the previous slice moved first-party `client_kind` names into Go protocol ownership.

## Approach

- Added a first-party `client_kind -> source_role` mapping to both protocol layers:
  - `internal/protocol/types.go`
  - `shared/protocol/src/index.ts`
- Extended Go protocol tests and Go↔TypeScript parity to pin that mapping directly.
- Switched daemon handshake validation to consume the protocol-owned mapping instead of a daemon-local switch.
- Switched `agent`, `inspector`, and `shared/chrome-sim` hello/source construction to consume the shared protocol mapping for their first-party source roles.

## Risk and mitigation

- Risk: moving one more handshake rule into protocol ownership could start to blur stable contract data with daemon-local policy.
- Mitigation: this slice centralizes only the identity mapping between first-party `client_kind` values and first-party `src.role` values. Capability scope decisions, unknown-client fallback, and extension-ID normalization remain daemon-local behavior.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run 'TestTypeScriptProtocolParity|TestFirstPartyClientKinds|TestFirstPartySourceRolesByClientKind|TestSourceRoleForClientKind' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/daemon -run 'TestWebSocketHandshakeSendsHelloAckAndTracksConnection|TestWebSocketHandshakeRejectsKnownClientKindWithUnexpectedSourceRole|TestWebSocketHandshakeScopesDevAgentCapabilitiesByClientKind|TestWebSocketHandshakeScopesChromeSimCapabilitiesByClientKind' -count=1`
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

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining first-party handshake constants should be promoted into a single shared artifact, or whether the remaining pieces are now daemon-local policy and should intentionally stay there.

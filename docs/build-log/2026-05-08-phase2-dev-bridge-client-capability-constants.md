# Phase 2 Dev Bridge daemon — shared first-party capability request contract

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The daemon now scopes negotiated capabilities by `client_kind`, but the three first-party TypeScript clients still declared their `capabilities_requested` sets separately. That left the handshake contract duplicated across `dev-agent`, `inspector`, and `chrome-sim`, so a future capability change could drift between packages even though the request sets are part of one shared bridge protocol.

## Approach

- Added a `negotiableCapabilityNames` list and `firstPartyRequestedCapabilities` map to `@panex/protocol` so first-party request sets live in one shared TypeScript protocol contract.
- Typed those request sets against the negotiable-capability list so response-only names such as `chrome.api.result`, `query.events.result`, and `query.storage.result` are not representable in first-party handshake requests.
- Switched `dev-agent`, `inspector`, and `chrome-sim` to use the shared request map instead of carrying package-local handshake capability arrays.
- Extended shared protocol tests to pin the scoped request sets and the exclusion of response-only message names from the negotiable capability surface.

## Risk and mitigation

- Risk: centralizing the constants could accidentally change request ordering or narrow a client’s negotiated surface.
- Mitigation: the shared protocol tests pin the exact first-party request sets, and the existing client handshake tests still verify the emitted `hello` payloads from each package.

## Verification

- `pnpm install --frozen-lockfile`
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
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by deciding whether the remaining daemon-side first-party capability scopes should also be exposed through a machine-readable artifact for cross-language drift detection, without prematurely coupling Go runtime wiring to TypeScript package layout.

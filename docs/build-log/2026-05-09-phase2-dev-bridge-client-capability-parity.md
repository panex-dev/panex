# Phase 2 Dev Bridge daemon — Go/TypeScript parity for first-party capability scopes

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The shared TypeScript protocol contract now owns the first-party `capabilities_requested` sets, while the daemon still owns the authoritative client-kind capability scopes in Go. Without a cross-language parity guard, those two sources could drift silently even though they describe the same first-party bridge contract.

## Approach

- Added a daemon-side parity test that reads `shared/protocol/src/index.ts` directly and compares:
  - `negotiableCapabilityNames` against the daemon’s full negotiable capability catalog.
  - `firstPartyClientKinds` against the known first-party client kinds enforced by the daemon.
  - `firstPartyRequestedCapabilities` against `supportedCapabilitiesForClientKind(...)` for `dev-agent`, `inspector`, and `chrome-sim`.
- Kept the change test-only so the runtime handshake and capability negotiation behavior remain untouched.

## Risk and mitigation

- Risk: the regex-based parity parser could become too brittle if the TypeScript constant formatting changes for cosmetic reasons only.
- Mitigation: the parser is scoped to the small exported constant shapes already used for existing protocol parity coverage, and the failure mode is explicit drift at test time rather than runtime behavior changes.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/daemon -run TestTypeScriptClientCapabilityParity -count=1`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by deciding whether first-party capability scopes should stay mirrored across Go and TypeScript with parity tests, or whether a later shared artifact/generator is justified once more cross-language contract surfaces accumulate.

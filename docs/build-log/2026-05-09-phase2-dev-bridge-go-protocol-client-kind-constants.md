# Phase 2 Dev Bridge daemon — centralize Go protocol client kinds and negotiable capability names

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The shared TypeScript protocol contract already named the first-party bridge `client_kind` values and negotiable capability catalog, but Go still treated those wire-level values as daemon-local string literals. That left the protocol package incomplete as the stable contract boundary and forced the daemon parity test to parse TypeScript directly instead of comparing its behavior against Go protocol ownership first.

## Approach

- Added Go protocol constants for the stable bridge identity surface in `internal/protocol/types.go`:
  - `NegotiableCapabilityNames`
  - `ClientKind`
  - `FirstPartyClientKinds`
- Extended protocol tests to pin those exported lists and to compare them against `shared/protocol/src/index.ts` in the existing TypeScript parity test.
- Switched daemon capability lists and client-kind checks to consume Go protocol constants instead of repeating protocol-stable strings locally.
- Retargeted daemon parity so `internal/daemon/capability_parity_test.go` compares daemon capability scoping against Go protocol ownership rather than reparsing the TypeScript file itself.

## Risk and mitigation

- Risk: moving protocol-stable names into Go could blur the line between stable contract and daemon-specific behavior.
- Mitigation: this slice centralizes only the stable identifiers (`client_kind` values and negotiable message names). The daemon still owns package-local policy such as which first-party client receives which subset and how unknown clients fall back.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/protocol -run 'TestTypeScriptProtocolParity|TestNegotiableCapabilityNames|TestFirstPartyClientKinds|TestEncodeDecodeRoundTrip|TestDecodePayloadTypedCompatibility' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/daemon -run 'TestTypeScriptClientCapabilityParity|TestWebSocketHandshakeSendsHelloAckAndTracksConnection|TestWebSocketHandshakeNegotiatesCapabilities|TestWebSocketHandshakeScopesDevAgentCapabilitiesByClientKind|TestWebSocketHandshakeScopesChromeSimCapabilitiesByClientKind|TestWebSocketHandshakeUnknownClientKindFallsBackToGlobalCapabilities|TestWebSocketCapabilityListsSeparateBroadcastAndHandler' -count=1`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining first-party handshake or routing rules still describe protocol-stable identities outside `internal/protocol`, and only centralize them further if they are true cross-package contract rather than daemon-local policy.

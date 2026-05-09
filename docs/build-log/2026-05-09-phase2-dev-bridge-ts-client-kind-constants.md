# Phase 2 Dev Bridge daemon — centralize named first-party client kind constants in shared protocol contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The stable first-party `client_kind` names were already centralized in Go protocol ownership, but the TypeScript bridge clients still reached for raw string literals when building `hello` envelopes. That left the first-party handshake identity surface partially centralized and made future drift in TypeScript emitter call sites easier than it needed to be.

## Approach

- Added named TypeScript first-party client kind constants to the shared protocol layer:
  - `shared/protocol/src/index.ts`
- Extended Go↔TypeScript parity so those exported TypeScript constants must still match the Go `ClientKind*` constants.
- Switched the TypeScript first-party hello emitters in `agent`, `inspector`, and `chrome-sim` to consume the shared constants instead of local string literals.
- Added focused protocol and emitter-level coverage for the named client-kind contract.

## Risk and mitigation

- Risk: adding one more set of shared TypeScript exports could look like unnecessary duplication beside the existing `firstPartyClientKinds` array.
- Mitigation: this slice keeps the array as the compatibility surface while adding direct named identifiers for emitter call sites. That removes raw literals without forcing the protocol parity logic or object-key structure to depend on computed properties.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
- `pnpm --dir shared/protocol test`
- `pnpm --dir agent test`
- `pnpm --dir shared/chrome-sim test`
- `pnpm --dir inspector run check`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining TypeScript-side first-party handshake metadata still relies on raw literals where a shared protocol identifier would be clearer and easier to keep in parity.

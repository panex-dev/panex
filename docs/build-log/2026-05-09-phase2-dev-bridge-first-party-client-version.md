# Phase 2 Dev Bridge daemon — centralize the shared first-party client version handshake contract

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The first-party Dev Bridge clients already sent the same `client_version: "dev"` placeholder in their `hello` handshakes, but that identity field was still duplicated independently in `agent`, `inspector`, and `chrome-sim`. That made the surfaced runtime metadata easy to drift across first-party clients even after the previous slices centralized the surrounding handshake identity contracts.

## Approach

- Added a protocol-owned first-party client version constant to both protocol layers:
  - `internal/protocol/types.go`
  - `shared/protocol/src/index.ts`
- Extended Go↔TypeScript parity so the exported TypeScript constant must still match the Go protocol constant.
- Switched the `agent`, `inspector`, and `chrome-sim` hello emitters to consume that shared protocol constant instead of package-local `"dev"` literals.
- Added focused protocol and first-party handshake coverage for the shared client-version contract.

## Risk and mitigation

- Risk: centralizing a placeholder client version could blur the line between stable first-party identity contracts and future package-specific release versioning.
- Mitigation: this slice centralizes only the currently shared first-party handshake placeholder that all three clients already emit today. If the project later needs package-specific semantic versions, that can replace this one shared constant intentionally rather than by accidental drift.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run 'TestTypeScriptProtocolParity|TestDefaultFirstPartyClientVersion' -count=1`
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

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining surfaced handshake metadata still relies on duplicated first-party literals instead of shared protocol contracts with parity coverage.

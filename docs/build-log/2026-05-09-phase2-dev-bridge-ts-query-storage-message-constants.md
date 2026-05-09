# Phase 2 Dev Bridge daemon — centralize named query and storage message constants in shared protocol contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The stable Dev Bridge query and storage message names already existed in Go as `MessageQueryEvents`, `MessageQueryResult`, `MessageQueryStorage`, `MessageStorageResult`, `MessageStorageDiff`, `MessageStorageSet`, `MessageStorageRemove`, and `MessageStorageClear`, but first-party TypeScript runtime code and focused fixtures still repeated those wire names inline. That left the inspector query/storage surface and the `chrome-sim` storage-diff path more drift-prone than the protocol contract needed to be.

## Approach

- Added named TypeScript exports for the query and storage message names in `@panex/protocol`.
- Extended Go↔TypeScript parity so those TypeScript constants must still match the Go query and storage message names.
- Switched the inspector query/storage runtime paths, capability gating, timeline summary helper, `chrome-sim` storage-diff transport path, and the focused protocol/inspector/chrome-sim fixtures to consume those shared constants instead of repeating the wire literals.

## Risk and mitigation

- Risk: the new TypeScript query/storage message exports could drift from the broader protocol tables or Go message-name contract if only one layer changes later.
- Mitigation: shared protocol tests now assert the named query/storage constants directly, and Go parity compares every new TypeScript export against the matching Go message name on every parity run.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
- `pnpm --dir shared/protocol test`
- `pnpm --dir shared/chrome-sim test`
- `pnpm --dir inspector test`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon contract cleanup by promoting the remaining build and reload message names to shared TypeScript protocol exports where first-party runtime code and fixtures still repeat those identifiers inline.

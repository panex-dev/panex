# Phase 2 Dev Bridge daemon — centralize named source role constants in shared protocol contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The shared TypeScript protocol already exported named first-party `client_kind` constants, but stable `src.role` values were still mostly repeated as raw strings at the TypeScript layer. That left the source-role side of the bridge identity contract less explicit than the rest of the protocol surface and made the inspector timeline filter and test fixtures easier to drift away from Go protocol ownership.

## Approach

- Added named TypeScript source-role constants to the shared protocol layer while keeping the existing `sourceRoles` array and first-party source-role map stable for current parity parsing and runtime consumers.
- Extended Go↔TypeScript protocol parity so the new exported TypeScript source-role constants must still match the Go `Source*` constants.
- Switched the inspector timeline filter UI and validation helpers to consume shared source-role constants instead of restating the role names inline.
- Updated focused protocol, inspector, and chrome-sim tests to assert against the shared source-role exports.

## Risk and mitigation

- Risk: adding named source-role exports alongside the existing `sourceRoles` array introduces one more place where TypeScript values could drift.
- Mitigation: shared protocol tests now assert the named source-role exports directly, and Go parity compares each exported TypeScript source-role constant against the Go `Source*` contract.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
- `pnpm --dir shared/protocol test`
- `pnpm --dir inspector test`
- `pnpm --dir shared/chrome-sim test`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon contract cleanup by checking whether the remaining first-party TypeScript protocol fixtures and UI filters can consume shared protocol exports directly instead of spelling the wire values inline.

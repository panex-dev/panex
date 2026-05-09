# Phase 2 Dev Bridge daemon — centralize named handshake lifecycle message constants in shared protocol contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The stable Dev Bridge handshake lifecycle message names already existed in Go as `MessageHello` and `MessageHelloAck`, but the TypeScript bridge clients and fixtures still repeated `"hello"` and `"hello.ack"` directly. That left the lifecycle handshake contract less explicit than the rest of the shared protocol surface and made emitter or fixture drift easier than necessary.

## Approach

- Added named TypeScript exports for the handshake lifecycle message names in `@panex/protocol`.
- Extended Go↔TypeScript parity so those new TypeScript constants must still match the Go `MessageHello` and `MessageHelloAck` values.
- Switched the first-party TypeScript hello emitters, the shared `isHelloAck` guard, and the focused agent/chrome-sim/protocol fixtures to consume those shared constants instead of repeating the wire literals.

## Risk and mitigation

- Risk: the new named handshake message exports could diverge from the existing `envelopeNames` array or Go message-name contract if only one side changes.
- Mitigation: shared protocol tests now assert the named handshake constants directly, and Go parity compares both new TypeScript exports against the Go lifecycle message names.

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

Continue the Phase 2 Dev Bridge daemon contract cleanup by checking whether the remaining commonly emitted protocol message names should be promoted to named TypeScript exports where first-party runtime code and fixtures still repeat them inline.

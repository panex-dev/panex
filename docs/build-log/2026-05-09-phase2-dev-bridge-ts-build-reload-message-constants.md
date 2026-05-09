# Phase 2 Dev Bridge daemon — centralize named build and reload message constants in shared protocol contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The shared Dev Bridge protocol still left `build.complete` and `command.reload` partially stringly typed on the TypeScript side. The stable Go message names already existed as `MessageBuildComplete` and `MessageCommandReload`, but first-party TypeScript protocol tables, the agent handshake capability check, the inspector timeline summary path, and focused fixtures still repeated those wire names inline.

## Approach

- Added named TypeScript exports for the build and reload message names in `@panex/protocol`.
- Extended `isReloadCommand(...)` to use the shared reload constant so first-party TypeScript code can consume one named export instead of repeating the wire literal.
- Extended Go↔TypeScript parity to compare the new TypeScript exports directly against the Go protocol message names.
- Switched the first-party runtime consumers in the dev-agent handshake and inspector timeline helper, plus the focused protocol/agent/inspector tests, to use the shared constants.

## Risk and mitigation

- Risk: the new constants could drift from the canonical Go protocol names or from the first-party consumers that depend on them.
- Mitigation: shared protocol tests now assert the build/reload constants directly, and the Go parity test fails if either new TypeScript export diverges from the canonical Go protocol message name.

## Verification

- `pnpm install --frozen-lockfile`
- `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
- `pnpm --dir shared/protocol test`
- `pnpm --dir agent test`
- `pnpm --dir inspector test`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon contract cleanup by centralizing the next remaining stable TypeScript protocol surface only where repeated first-party wire literals still cross the package boundary.

# Phase 2 Dev Bridge daemon — centralize named Chrome API transport message constants in shared protocol contracts

**Status:** PR pending
**Date:** 2026-05-09

## Problem

The stable Dev Bridge Chrome API transport names already existed in Go as `MessageChromeAPICall`, `MessageChromeAPIResult`, and `MessageChromeAPIEvent`, but the TypeScript runtime code and focused fixtures still repeated `"chrome.api.call"`, `"chrome.api.result"`, and `"chrome.api.event"` inline across `shared/chrome-sim` and the inspector. That kept a heavily used protocol sub-family less explicit than the rest of the shared contract surface and made cross-runtime drift easier than necessary.

## Approach

- Added named TypeScript exports for the Chrome API transport message names in `@panex/protocol`.
- Extended Go↔TypeScript parity so those TypeScript constants must still match the Go Chrome API message names.
- Switched the `chrome-sim` transport/runtime, the inspector Chrome API activity and replay decoding paths, the inspector runtime-send command emitter, and the focused protocol/chrome-sim/inspector fixtures to consume those shared constants instead of repeating the wire literals.

## Risk and mitigation

- Risk: the new TypeScript Chrome API message exports could drift from the broader protocol tables or Go message-name contract if only one layer changes later.
- Mitigation: shared protocol tests now assert the named Chrome API constants directly, and Go parity compares all three TypeScript exports against the Go Chrome API message names on every parity run.

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

Continue the Phase 2 Dev Bridge daemon contract cleanup by promoting the remaining build/query/storage message names to named TypeScript exports where first-party runtime code and fixtures still repeat those wire identifiers inline.

# Phase 2 Dev Bridge daemon — centralize remaining inspector runtime Chrome API capability constants

**Status:** PR pending
**Date:** 2026-05-10

## Problem

Most first-party TypeScript runtime consumers had already moved onto shared Dev Bridge protocol constants, but the inspector still repeated the `chrome.api.call` capability name inline in its runtime send gate and in the Workbench and Probe History capability-gating UI. That left a few last runtime callsites on string literals even though the shared protocol already exported `CHROME_API_CALL_MESSAGE_NAME`.

## Approach

- Switched the inspector connection runtime-send gate to use `CHROME_API_CALL_MESSAGE_NAME`.
- Switched the Probe History and Workbench tabs to use the same shared constant for both negotiated-capability checks and the displayed protocol-name hint text.
- Recorded the follow-up in the build log and status tracker.

## Risk and mitigation

- Risk: this could accidentally change the inspector’s runtime-probe availability checks instead of only centralizing the identifier source.
- Mitigation: the PR only swaps existing literal uses to the shared constant and leaves the surrounding gating logic unchanged; full typecheck, tests, and root verification still exercise the same send/replay paths.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon cleanup by removing the next remaining repeated first-party runtime protocol identifier only where it still crosses package boundaries.

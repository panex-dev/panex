# Phase 2 Dev Bridge daemon — centralize remaining inspector fixture message constants

**Status:** PR pending
**Date:** 2026-05-10

## Problem

After the earlier protocol constant cleanup, several focused inspector fixtures still repeated stable Dev Bridge wire names inline instead of consuming the shared `@panex/protocol` exports. That left replay, activity-log, workbench, and connection capability tests drifting from the contract source of truth even though runtime code had already been centralized.

## Approach

- Switched the remaining raw inspector replay fixtures to `CHROME_API_RESULT_MESSAGE_NAME` and `CHROME_API_EVENT_MESSAGE_NAME`.
- Switched the inspector activity-log and workbench fixtures to the shared build, query, storage, and websocket URL constants they exercise.
- Switched the inspector connection capability assertion to `CHROME_API_RESULT_MESSAGE_NAME` so the response-only capability exclusion check stays tied to the shared protocol export.

## Risk and mitigation

- Risk: fixture cleanup could accidentally change the inspector test intent instead of only centralizing the identifier source.
- Mitigation: the PR only replaces inline protocol literals with the already-exported constants, and the focused inspector tests plus the full repo verification still exercise the same behaviors and request gating.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge contract cleanup by removing the next remaining repeated stable protocol literal from first-party runtime code or focused cross-package fixtures.

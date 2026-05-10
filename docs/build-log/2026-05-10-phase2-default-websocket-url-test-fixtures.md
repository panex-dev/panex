# Phase 2 Dev Bridge daemon — centralize remaining default websocket URL test fixtures

**Status:** PR pending
**Date:** 2026-05-10

## Problem

After the runtime and protocol cleanup, several first-party test fixtures still repeated the default daemon websocket URL inline instead of consuming the shared `DEFAULT_DAEMON_WEBSOCKET_URL` export. That left agent, inspector preview, and chrome-sim fixture coverage restating the transport contract rather than validating the shared source of truth.

## Approach

- Switched the remaining agent test fixtures that referenced the default websocket URL to `DEFAULT_DAEMON_WEBSOCKET_URL`.
- Switched the inspector preview-injection fixtures to the shared default websocket URL export.
- Switched the remaining chrome-sim bootstrap and transport fixtures to the shared default websocket URL export.

## Risk and mitigation

- Risk: fixture-only cleanup could accidentally change test setup instead of only centralizing the default URL source.
- Mitigation: the PR only replaces the repeated default URL literal with the existing shared export, and the full repo verification still exercises the same websocket bootstrap and transport paths.

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

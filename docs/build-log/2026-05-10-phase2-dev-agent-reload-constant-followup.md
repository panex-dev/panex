# Phase 2 Dev Bridge daemon — centralize remaining dev-agent reload runtime constants

**Status:** PR pending
**Date:** 2026-05-10

## Problem

After the broader TypeScript message-name cleanup, the dev-agent still had one last runtime reload consumer on a raw wire literal: the diagnostic event emitted from the background websocket handler. The matching focused diagnostics and reload fixtures also still repeated `command.reload` and `build.complete` inline instead of consuming the shared protocol exports.

## Approach

- Switched the dev-agent background reload diagnostic event to use `COMMAND_RELOAD_MESSAGE_NAME`.
- Switched the focused diagnostics and reload tests to consume the shared `COMMAND_RELOAD_MESSAGE_NAME` and `BUILD_COMPLETE_MESSAGE_NAME` exports.
- Recorded the follow-up in the build log and status tracker.

## Risk and mitigation

- Risk: this could accidentally change the dev-agent diagnostic event label or the reload test intent instead of only centralizing the identifier source.
- Mitigation: the PR only replaces existing string literals with the already-shared protocol constants, and the full agent plus repo-wide verification still exercises the same reload and diagnostic paths.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon cleanup by removing the next remaining repeated first-party protocol identifier only where runtime code or focused contract fixtures still cross package boundaries.

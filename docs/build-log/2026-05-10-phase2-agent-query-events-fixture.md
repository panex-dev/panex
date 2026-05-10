# Phase 2 Dev Bridge daemon — centralize remaining agent query-events fixture

**Status:** PR pending
**Date:** 2026-05-10

## Problem

After the broader protocol-constant cleanup, the dev-agent handshake tests still repeated `query.events` inline in the negative negotiated-capability fixture. That left the last first-party capability-name fixture restating the wire contract instead of consuming the shared protocol export it is meant to validate against.

## Approach

- Switched the dev-agent handshake negative capability fixture to `QUERY_EVENTS_MESSAGE_NAME`.
- Recorded the follow-up in the build log and status tracker.

## Risk and mitigation

- Risk: fixture cleanup could accidentally change the handshake-negotiation test intent instead of only centralizing the capability identifier source.
- Mitigation: the PR only replaces the repeated capability string with the existing shared export, and the agent handshake plus full repo verification still exercise the same rejection path when `command.reload` is absent.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge contract cleanup only if another repeated stable protocol literal appears in first-party runtime code or focused cross-package fixtures.

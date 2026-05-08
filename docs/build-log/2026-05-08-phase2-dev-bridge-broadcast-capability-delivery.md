# Phase 2 Dev Bridge daemon — negotiated broadcast delivery

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The daemon already enforced negotiated capabilities for client-originated handler commands, but daemon-originated broadcasts still ignored the negotiated capability set. Live `build.complete`, `command.reload`, `storage.diff`, and `chrome.api.event` frames were filtered only by extension targeting, so sessions could still receive bridge traffic they had never negotiated. The inspector also relied on live build and reload traffic in the Timeline without ever requesting those broadcast capabilities, so tightening daemon delivery would have regressed that operator surface unless the consumer negotiated what it actually uses.

## Approach

- Tightened the daemon live-delivery filter so a session only receives daemon-originated broadcast messages when that exact message name was negotiated for the session, then kept the existing extension targeting on top of that rule.
- Added daemon regression coverage for both untargeted and targeted broadcasts so sessions that did not negotiate a broadcast capability stop receiving that traffic, even when they otherwise match the extension target.
- Updated the inspector handshake request set to include the live broadcast capabilities its Timeline already consumes (`build.complete` and `command.reload`) and added a focused inspector regression test to pin that contract.
- Corrected daemon integration and websocket tests that had been implicitly depending on the old over-broadcast behavior for dev-agent sessions.

## Risk and mitigation

- Risk: tightening daemon broadcast delivery could silently starve a legitimate client that was relying on previously over-broadcasted traffic.
- Mitigation: this slice pairs the daemon-side delivery change with the matching inspector capability request fix and exercises the affected live-delivery paths in daemon unit/integration tests and inspector tests.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking the remaining bridge consumers for any other live message or command path that still assumes broader session capabilities than it explicitly negotiates.

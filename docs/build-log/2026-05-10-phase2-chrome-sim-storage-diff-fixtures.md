# Phase 2 Dev Bridge daemon — centralize remaining chrome-sim storage diff fixtures

**Status:** PR pending
**Date:** 2026-05-10

## Problem

After the broader protocol-constant cleanup, the shared chrome-sim transport tests still repeated `storage.diff` inline in their negotiated capability fixtures. That left the browser-bridge transport coverage partially restating the storage-diff contract instead of consuming the shared protocol export it is meant to pin.

## Approach

- Switched the chrome-sim transport handshake fixture to request `STORAGE_DIFF_MESSAGE_NAME`.
- Switched the no-`chrome.api.call` hello-ack fixture to use `STORAGE_DIFF_MESSAGE_NAME`.
- Switched the shared hello-ack builder fixture to use `STORAGE_DIFF_MESSAGE_NAME` in its default negotiated capabilities.

## Risk and mitigation

- Risk: fixture cleanup could accidentally change the negotiated-capability intent instead of only centralizing the storage-diff identifier source.
- Mitigation: the PR only replaces the repeated string literal with the existing shared protocol export, and the chrome-sim transport plus full repo verification still exercise the same handshake and capability-gating paths.

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

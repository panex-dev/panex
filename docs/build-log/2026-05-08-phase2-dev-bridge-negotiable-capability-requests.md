# Phase 2 Dev Bridge daemon — negotiable capability requests only

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The first-party bridge clients still requested `chrome.api.result` during `hello`, even though `chrome.api.result` is a daemon response event rather than a negotiated session capability. That made the handshake request sets broader and noisier than the actual bridge contract, obscured which capabilities the daemon was expected to negotiate, and forced tests to normalize a response-only message name as if it belonged in capability negotiation.

## Approach

- Removed `chrome.api.result` from the inspector and `chrome-sim` handshake request sets so they advertise only capabilities the daemon can actually negotiate.
- Kept the runtime behavior unchanged for `chrome.api.result` delivery, because daemon responses to `chrome.api.call` are written directly to the calling session and do not depend on handshake capability negotiation.
- Updated `chrome-sim` handshake fixtures to model realistic negotiated capability sets and added request-set assertions so the browser transport no longer regresses toward response-only capability names.
- Extended the inspector request-set test to assert both the required live/command capabilities and the absence of `chrome.api.result`.

## Risk and mitigation

- Risk: a test or client helper might have been implicitly treating response events as negotiable capabilities and fail once the request set became stricter.
- Mitigation: the slice updates the transport fixtures and adds direct assertions for both clients’ request sets, so the narrower contract is explicit instead of incidental.

## Verification

- `pnpm install --frozen-lockfile`
- `pnpm --dir inspector test`
- `pnpm --dir shared/chrome-sim test`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by checking whether any remaining first-party handshake constants can be derived from one shared negotiable-capability definition without widening package boundaries prematurely.

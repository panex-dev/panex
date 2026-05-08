# Phase 2 Dev Bridge daemon — capability-aware chrome-sim calls

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The browser-side `shared/chrome-sim` transport completed `hello` / `hello.ack`, but it still treated negotiated capabilities as advisory. After handshake it would send `chrome.api.call` whenever a consumer invoked `call()`, even if the daemon had not negotiated `chrome.api.call` for that session. That turned a legitimate narrower bridge capability set into an avoidable policy-violation disconnect.

## Approach

- Recorded the negotiated capability set from `hello.ack` inside the chrome-sim transport lifecycle.
- Cleared that capability state on disconnect so reconnects always reflect the current daemon handshake.
- Rejected `call()` locally with a clear error when the daemon did not negotiate `chrome.api.call`, instead of sending an unsupported command.
- Added transport regression coverage for the missing-capability case.

## Risk and mitigation

- Risk: the transport could retain stale capability state across reconnects and either over-allow or over-reject calls.
- Mitigation: the transport clears negotiated capabilities on socket close and repopulates them only from the next successful `hello.ack`.

## Verification

- `pnpm install --frozen-lockfile`
- `pnpm --dir shared/chrome-sim test`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by making any remaining bridge-side consumers respect negotiated capabilities only at the concrete command paths they actually use. This PR keeps the browser transport from turning an unsupported `chrome.api.call` into a disconnect.

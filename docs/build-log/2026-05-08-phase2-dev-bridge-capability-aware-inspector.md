# Phase 2 Dev Bridge daemon — capability-aware inspector follow-ups

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The inspector had started recording negotiated bridge capabilities from `hello.ack`, but it still treated them as display-only metadata. After handshake it always sent `query.events` and `query.storage`, and its storage/runtime controls stayed enabled whenever the websocket was open. That meant a daemon that legitimately negotiated a narrower capability set would be met with unsupported follow-up commands, and the bridge would close the session with a policy violation.

## Approach

- Added inspector-side capability helpers so negotiated bridge capabilities can be checked consistently from connection logic and UI tabs.
- Guarded post-`hello.ack` follow-up queries and all outbound inspector bridge commands behind the negotiated capability set, so unsupported commands are not sent.
- Updated the Timeline, Storage, Workbench, and Probe History tabs to disable unsupported actions and explain when a session did not negotiate the required capability instead of implying the websocket is simply closed.
- Added regression coverage for capability checks and conditional post-handshake follow-up queries.

## Risk and mitigation

- Risk: the inspector could drift into multiple ad hoc capability checks and leave one command path unguarded.
- Mitigation: the connection layer now owns the send-side capability guard, and the added tests cover both capability matching and the exact follow-up query set emitted after `hello.ack`.

## Verification

- `pnpm install --frozen-lockfile`
- `pnpm --dir inspector test`

## Next Step

Continue the Phase 2 Dev Bridge daemon milestone by making additional bridge consumers capability-aware only where there is a concrete command path or operator surface behind them. This PR focuses on preventing the inspector from turning negotiated capability limits into disconnects.

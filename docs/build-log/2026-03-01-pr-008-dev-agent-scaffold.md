# PR8 - Dev Agent Scaffold (MV3) and command.reload Handling

## Metadata
- Date: 2026-03-01
- PR: 8
- Branch: `feat/dev-agent-scaffold`
- Title: scaffold dev agent chrome extension and handle command.reload
- Commit(s): pending

## Problem
- The daemon could emit protocol events, but there was no Chrome-side agent to consume commands.
- Without an extension runtime bridge, the save-to-reload loop could not be completed end-to-end.

## Approach (with file+line citations)
- Added isolated Dev Agent package scaffold with deterministic scripts and pinned dependencies:
  - `agent/package.json:2-21`
  - `agent/pnpm-lock.yaml:1-420`
  - `agent/tsconfig.json:2-13`
- Added MV3 extension manifest and build pipeline for a bundled background worker:
  - `agent/manifest.json:2-12`
  - `agent/scripts/build.mjs:1-19`
- Implemented runtime config loading from `chrome.storage.local` and daemon URL/token construction:
  - `agent/src/config.ts:1-38`
- Implemented protocol envelope/types and boundary guard for decoded payloads:
  - `agent/src/protocol.ts:1-52`
- Implemented background WebSocket runtime:
  - handshake hello emission in `agent/src/background.ts:22-37`
  - malformed payload rejection and reload dispatch in `agent/src/background.ts:39-55`
  - bounded reconnect backoff in `agent/src/background.ts:66-75`
- Implemented command handler that maps `command.reload` to `chrome.runtime.reload()`:
  - `agent/src/reload.ts:1-14`
- Added tests for command handling, config normalization, URL token replacement, and envelope guards:
  - `agent/tests/reload.test.ts:1-53`
  - `agent/tests/config.test.ts:1-65`
  - `agent/tests/protocol.test.ts:1-32`
- Updated repository ignore rules for Node/pnpm artifacts:
  - `.gitignore:30-34`

## Risk and Mitigation
- Risk: MV3 service worker suspension can cause reconnect flapping during daemon restarts.
- Mitigation: exponential reconnect backoff with ceiling in `agent/src/background.ts:66-75`.
- Risk: malformed wire payloads could crash runtime handlers.
- Mitigation: decode try/catch + envelope shape guard in `agent/src/background.ts:44-52` and `agent/src/protocol.ts:35-52`.

## Verification
- Agent checks:
  - `cd agent && pnpm run check`
  - `cd agent && pnpm run test`
  - `cd agent && pnpm run build`
- Repo checks:
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`

## Teach-back (engineering lessons)
- Treat extension background workers as volatile runtimes: reconnect strategy is not optional.
- Protocol boundary guards should validate transport shape first, then let feature handlers own deeper payload semantics.
- Keeping the agent in an isolated package with its own lockfile prevents toolchain drift from leaking into Go workflows.

## Next Step
- Wire daemon `build.complete` success path to emit `command.reload` so saves trigger extension reload automatically.

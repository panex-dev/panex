# PR38 - `shared/chrome-sim` Transport + Storage Shim Scaffold

## Metadata
- Date: 2026-03-05
- PR: 38
- Branch: `feat/pr38-chrome-sim-transport-scaffold`
- Title: add browser-side chrome-sim transport scaffold with storage namespace wiring
- Commit(s): pending

## Problem
- PR37 implemented daemon routing for `chrome.api.call`, but there was no reusable browser-side shim package to initiate handshake, send correlated calls, and expose `chrome.storage.*` through the new protocol.
- Without a dedicated client package, simulator usage would stay ad-hoc and the storage simulation path would not be portable across preview surfaces.

## Approach (with file+line citations)
- Created a new `@panex/chrome-sim` package scaffold with strict TS config and local quality scripts:
  - package exports/scripts/deps:
  - `shared/chrome-sim/package.json:1-26`
  - TS strict/noEmit compilation scope:
  - `shared/chrome-sim/tsconfig.json:1-14`
- Implemented websocket transport contract with handshake, `call_id` correlation, timeout handling, event subscription, and reconnect backoff:
  - transport surface + options:
  - `shared/chrome-sim/src/transport.ts:13-74`
  - hello/hello.ack handshake and lifecycle transitions:
  - `shared/chrome-sim/src/transport.ts:167-267`
  - `chrome.api.call` send path and pending call timeout map:
  - `shared/chrome-sim/src/transport.ts:295-345`
  - result/event envelope routing + reconnect scheduling:
  - `shared/chrome-sim/src/transport.ts:94-165`
  - `shared/chrome-sim/src/transport.ts:252-293`
- Added storage namespace adapter and installation entrypoint for `window.chrome.storage.{local,sync,session}`:
  - storage namespace method mapping to transport calls:
  - `shared/chrome-sim/src/storage.ts:3-56`
  - browser install shim for storage namespaces:
  - `shared/chrome-sim/src/index.ts:16-55`
  - namespace registry scaffold for future unsupported-path instrumentation:
  - `shared/chrome-sim/src/registry.ts:1-57`
- Added tests for transport lifecycle/correlation/reconnect/events, storage adapters, reconnect math, and installation behavior:
  - `shared/chrome-sim/tests/transport.test.ts:8-277`
  - `shared/chrome-sim/tests/storage.test.ts:7-78`
  - `shared/chrome-sim/tests/reconnect.test.ts:6-15`
  - `shared/chrome-sim/tests/index.test.ts:7-40`

## Risk and Mitigation
- Risk: transport state bugs (stale sockets, unresolved calls, reconnect loops) can deadlock simulator calls.
- Mitigation: pending calls are tracked and rejected on timeout/disconnect/close, with explicit reconnect backoff and handshake timeout behavior (`shared/chrome-sim/src/transport.ts:82-109`, `shared/chrome-sim/src/transport.ts:192-195`, `shared/chrome-sim/src/transport.ts:252-265`, `shared/chrome-sim/src/transport.ts:325-337`).
- Risk: non-conforming daemon responses could leak invalid shapes into the shim API.
- Mitigation: storage adapter normalizes non-object get payloads and throws on invalid numeric responses for `getBytesInUse` (`shared/chrome-sim/src/storage.ts:44-56`).

## Verification
- Commands run:
  - `cd shared/chrome-sim && pnpm install`
  - `cd shared/chrome-sim && pnpm run check`
  - `cd shared/chrome-sim && pnpm run test`
- Expected:
  - dependencies install successfully with a generated lockfile.
  - typecheck passes (`tsc --noEmit`).
  - transport/storage/index/reconnect test suites pass.

## Teach-back (engineering lessons)
- Packaging simulator wiring as a dedicated module keeps protocol transport concerns isolated from UI surfaces.
- Correlated async RPC over websocket needs explicit pending-call lifecycle ownership (timeout + disconnect cleanup) to avoid hidden hangs.
- Early scaffolding tests for reconnect and envelope routing reduce risk before adding more chrome namespaces.

## Next Step
- Integrate `@panex/chrome-sim` into preview injection/bootstrap so extension surfaces use the shim automatically, then extend namespaces beyond `storage.*`.

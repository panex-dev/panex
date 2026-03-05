# PR37 - Daemon `chrome.api.call` Storage Router + Correlated `chrome.api.result`

## Metadata
- Date: 2026-03-05
- PR: 37
- Branch: `feat/pr37-daemon-chrome-api-storage-router`
- Title: implement daemon-side simulator router for storage `chrome.api.call` operations
- Commit(s): pending

## Problem
- PR36 added simulator transport envelopes, but the daemon had no execution path for `chrome.api.call`.
- Without daemon routing, clients cannot issue storage simulation calls (`get/set/remove/clear/getBytesInUse`) through the protocol or receive correlated `chrome.api.result`.

## Approach (with file+line citations)
- Extended daemon capability advertisement to include simulator transport capabilities:
  - `internal/daemon/websocket_server.go:27-38`
- Added `chrome.api.call` handling in websocket command router:
  - decode `chrome.api.call`, execute storage router, and respond with `chrome.api.result` to the caller session:
  - `internal/daemon/websocket_server.go:348-513`
- Implemented storage simulator router methods for namespace + method dispatch:
  - command validation (`call_id`, namespace, method) and operation routing:
  - `internal/daemon/websocket_server.go:516-623`
  - namespace mapping:
  - `internal/daemon/websocket_server.go:625-636`
  - storage operations:
  - `internal/daemon/websocket_server.go:638-759`
  - argument parsing/normalization helpers:
  - `internal/daemon/websocket_server.go:761-901`
- Added daemon websocket tests for simulator storage call/result behavior:
  - set/get/remove/clear with diff fanout + correlated result:
  - `internal/daemon/websocket_server_test.go:680-820`
  - `getBytesInUse` and default-object `get` semantics:
  - `internal/daemon/websocket_server_test.go:822-899`
  - unsupported namespace returns failure result without closing connection:
  - `internal/daemon/websocket_server_test.go:901-957`
  - missing `call_id` remains a protocol violation (connection close):
  - `internal/daemon/websocket_server_test.go:959-999`
  - reusable test helpers for envelope/result assertions:
  - `internal/daemon/websocket_server_test.go:1292-1353`

## Risk and Mitigation
- Risk: unsupported/invalid simulator calls could close healthy websocket sessions and degrade dev UX.
- Mitigation: unsupported namespace/method and bad method args return `chrome.api.result` with `success=false` instead of failing the connection (`internal/daemon/websocket_server.go:535-542`, `internal/daemon/websocket_server.go:544-620`).
- Risk: key/object argument ambiguity can create nondeterministic storage behavior.
- Mitigation: argument parsers normalize and validate supported shapes (string, string list, defaults object) with deterministic key ordering (`internal/daemon/websocket_server.go:761-901`).

## Verification
- Commands run:
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -count=1`
  - `go test ./internal/daemon -count=1 -run 'TestWebSocketChromeAPICall|TestWebSocketStorage|TestWebSocketQueryStorage'`
  - `go test ./internal/daemon -count=1`
- Expected:
  - protocol tests pass.
  - daemon websocket suite passes including new `chrome.api.call` scenarios.

## Teach-back (engineering lessons)
- Protocol messages only become real features once call routing + error semantics are implemented in the runtime.
- Returning structured failure results for unsupported simulator calls is safer than transport-level failure because clients can degrade gracefully.
- Parsing and normalizing API-like arguments in one place reduces drift between methods and keeps future shim wiring predictable.

## Next Step
- Implement client-side `shared/chrome-sim` transport scaffold (`call_id` correlation + timeout/reconnect) and route `chrome.storage.*` calls through `chrome.api.call`.

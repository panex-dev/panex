# PR31 - Thread Request Context Through Store Calls

## Metadata
- Date: 2026-03-04
- PR: 31
- Branch: `fix/pr31-store-context-threading`
- Title: thread context.Context through store and broadcast operations
- Commit(s): pending

## Problem
- Store operations (Append, Recent) in `websocket_server.go` used `context.Background()`, giving SQLite calls no cancellation path.
- If the store hangs, nothing could stop it — the server would block indefinitely.
- Audit §3 flagged this as an engineering quality gap.

## Approach (with file+line citations)
- Added `context.Context` parameter to `Broadcast`:
  - Why: store.Append inside Broadcast now respects caller's cancellation
  - Where: `internal/daemon/websocket_server.go:297`
- Added `context.Context` parameter to `handshake`:
  - Why: hello and hello.ack persistence now use the HTTP request context
  - Where: `internal/daemon/websocket_server.go:202`
- Added `context.Context` parameter to `handleClientMessage`:
  - Why: query.events Recent() call now has a cancellation path
  - Where: `internal/daemon/websocket_server.go:338`
- Added `context.Context` parameter to storage mutation APIs:
  - Why: SetStorageItem, RemoveStorageItem, ClearStorageArea now thread context through broadcastStorageDiff → Broadcast → store.Append
  - Where: `internal/daemon/websocket_server.go:428,456,486`
- Updated `envelopeBroadcaster` interface:
  - Where: `cmd/panex/main.go:38`
- Updated all callers in `cmd/panex/main.go` and test files:
  - Where: `cmd/panex/main.go:215,236`, `cmd/panex/main_test.go:429`, `internal/daemon/websocket_server_test.go` (all Broadcast/SetStorageItem/RemoveStorageItem/ClearStorageArea calls)

## Risk and Mitigation
- Risk: breaking change to public Broadcast/SetStorageItem/RemoveStorageItem/ClearStorageArea APIs.
- Mitigation: all callers updated in the same commit. No external consumers exist yet.
- Risk: readLoop still passes context.Background() to handleClientMessage since it outlives the HTTP request.
- Mitigation: readLoop terminates when the connection drops, so context.Background() is appropriate — the connection lifecycle is the cancellation boundary.

## Verification
- Commands run:
  - `make fmt && make lint && make test && make build`
- All tests pass with context threading.

## Teach-back (engineering lessons)
- Design lesson: context.Context should be threaded from the outermost caller inward at API design time, not retrofitted. Adding it later requires updating every call site.
- Testing lesson: test helpers that call public APIs (like fakeBroadcaster) need to track interface changes.

## Next Step
- Add WebSocket origin validation to restrict connections to localhost.

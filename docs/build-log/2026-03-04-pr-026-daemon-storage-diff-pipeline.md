# PR26 - Daemon Storage Mutation Pipeline and `storage.diff` Fanout

## Metadata
- Date: 2026-03-04
- PR: 26
- Branch: `feat/pr26-daemon-storage-diff-pipeline`
- Title: persist daemon storage state and broadcast `storage.diff` on mutations
- Commit(s): pending

## Problem
- `query.storage` existed, but responses were static empty snapshots and did not reflect runtime storage state.
- `storage.diff` was defined in protocol and consumed by inspector UI, but daemon had no mutation path that emitted those events.

## Approach (with file+line citations)
- Extended daemon capabilities and server state to include storage diff support and persistent in-memory storage areas:
  - `internal/daemon/websocket_server.go:23-30`
  - `internal/daemon/websocket_server.go:46-55`
  - `internal/daemon/websocket_server.go:90-100`
- Replaced static snapshot stub with state-backed `query.storage` snapshots:
  - `internal/daemon/websocket_server.go:385-418`
  - `internal/daemon/websocket_server.go:513-539`
- Added daemon mutation APIs (`SetStorageItem`, `RemoveStorageItem`, `ClearStorageArea`) that emit `storage.diff` envelopes through existing broadcast/event-store flow:
  - `internal/daemon/websocket_server.go:423-510`
  - `internal/daemon/websocket_server.go:541-557`
- Added helpers for area normalization and safe map cloning:
  - `internal/daemon/websocket_server.go:559-582`
- Added daemon integration tests for:
  - querying mutated storage state,
  - live `storage.diff` broadcast on mutation,
  - mutation input validation:
  - `internal/daemon/websocket_server_test.go:349-519`
- Advanced build-log tracker to PR26 completion and PR27 next target:
  - `docs/build-log/STATUS.md:29-35`
  - `docs/build-log/README.md:45-46`

## Risk and Mitigation
- Risk: concurrent reads/mutations could race and leak shared map references.
- Mitigation: storage state is guarded with `storageMu` and query snapshots clone area maps before returning (`internal/daemon/websocket_server.go:53-55`, `internal/daemon/websocket_server.go:518-539`, `internal/daemon/websocket_server.go:573-582`).
- Risk: invalid areas/keys could produce undefined mutation behavior.
- Mitigation: mutation APIs validate area and non-empty key and return explicit errors (`internal/daemon/websocket_server.go:423-472`, `internal/daemon/websocket_server_test.go:445-464`).

## Verification
- Commands run:
  - `go test ./internal/daemon -count=1`
  - `go test -race ./internal/daemon -count=1`
  - `go test ./internal/protocol -count=1`
  - `go test -race -count=1 ./...`
- Expected:
  - daemon tests pass with storage mutation coverage.
  - race tests pass with new storage locking paths.
  - protocol and full test suites stay green.

## Teach-back (engineering lessons)
- Protocol contracts become operational only when query and mutation paths are implemented in the same subsystem.
- Keeping mutation fanout on top of the existing `Broadcast` path avoids duplicating delivery and persistence logic.
- Small exported mutation APIs unblock later transport wiring without prematurely coupling daemon state logic to a specific caller.

## Next Step
- Implement PR27 transport wiring so storage simulation operations call daemon mutation APIs and drive `storage.diff` from real mutation traffic.

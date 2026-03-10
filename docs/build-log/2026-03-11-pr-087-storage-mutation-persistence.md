# PR87 - Storage Mutation Persistence

## Metadata
- Date: 2026-03-11
- PR: 87
- Branch: `feat/pr87-storage-mutation-persistence`
- Title: persist storage mutations transactionally across daemon restarts
- Commit(s):
  - `fix(store): persist storage mutations across restarts (#87)`

## Problem
- The daemon already persisted protocol history to SQLite, but storage snapshots still lived only in an in-memory map.
- That meant `query.storage` lost all state on daemon restart even though prior `storage.diff` events remained in the timeline, which broke restart correctness for Workbench presets and other storage-backed tooling.

## Approach (with file+line citations)
- Change 1:
  - Why: extend the SQLite-backed store so it owns storage snapshots in a dedicated table and persists storage mutation diffs into the existing event history in the same transaction.
  - Where: `internal/store/sqlite_event_store.go:57-84`
  - Where: `internal/store/sqlite_storage_state.go:16-359`
- Change 2:
  - Why: route daemon storage queries and mutations through the store instead of the in-memory map so restart behavior is correct by construction.
  - Where: `internal/daemon/websocket_server.go:56-95`
  - Where: `internal/daemon/websocket_server.go:429-558`
  - Where: `internal/daemon/websocket_server.go:841-964`
  - Where: `internal/daemon/websocket_server.go:1261-1331`
- Change 3:
  - Why: lock in the restart guarantee with store-level reopen coverage and an end-to-end daemon restart test over the real `query.storage` and `query.events` protocol surface.
  - Where: `internal/store/sqlite_event_store_test.go:151-267`
  - Where: `internal/daemon/integration_test.go:230-349`
  - Where: `internal/daemon/websocket_server_test.go:1963-2010`
- Change 4:
  - Why: close the runtime-safety follow-on in the tracker and move `Next` to the remaining timeline-scalability slice.
  - Where: `docs/build-log/STATUS.md:84-100`
  - Where: `docs/build-log/README.md:44-48`

## Risk and Mitigation
- Risk: moving storage state into SQLite could drift the live daemon behavior from the persisted event history if storage writes and diff persistence happen in separate steps.
- Mitigation: the store writes the storage row changes and the `storage.diff` event in the same SQLite transaction for each mutation.
- Risk: switching `query.storage` to the store could change visible value shapes or ordering unexpectedly.
- Mitigation: storage snapshots still return the same area names, preserve empty-area responses, and keep clear-diff key ordering under test.

## Verification
- Commands run:
  - `GOCACHE=/tmp/go-build go test ./internal/store -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1 -run 'TestIntegrationStoragePersistsAcrossDaemonRestart|TestIntegrationDaemonLifecycle|TestWebSocketStorage|TestWebSocketChromeAPICall'`
  - `make fmt`
  - `CI=1 pnpm install --frozen-lockfile`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Reopened the same SQLite file in tests and confirmed both storage snapshots and persisted `storage.diff` history survive daemon restart.

## Teach-back
- Design lesson: once a runtime already has one persisted state boundary, extending that same boundary is usually safer than inventing a second cache-plus-recovery path.
- Testing lesson: restart guarantees need at least one real reopen test; positive in-process mutation tests are not enough.
- Workflow lesson: follow-on queues stay useful only when closing one item also advances `Next` to the most relevant remaining slice.

## Next Step
- Deepen Timeline scalability beyond the current render-window cap so large persisted histories remain usable after the current windowing slice.

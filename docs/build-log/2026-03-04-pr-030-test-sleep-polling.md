# PR30 - Replace time.Sleep in Tests with Polling+Deadline

## Metadata
- Date: 2026-03-04
- PR: 30
- Branch: `fix/pr30-test-sleep-polling`
- Title: replace fragile time.Sleep synchronization in tests with polling
- Commit(s): pending

## Problem
- `file_watcher_test.go` used fixed 30ms sleeps to wait for the watcher goroutine to start. On slow CI runners, this races and causes flaky failures.
- `esbuild_builder_test.go` used a 1ms sleep between build ID calls when the atomic sequence counter already guarantees uniqueness.
- Audit §6 flagged timing-dependent tests as a CI flakiness risk.

## Approach (with file+line citations)
- Added `ready` channel to FileWatcher for test synchronization:
  - Why: lets tests block until Run() enters its event loop, replacing fixed-duration startup sleeps
  - Where: `internal/daemon/file_watcher.go:27` (field), `internal/daemon/file_watcher.go:74-76` (close signal)
- Added `startWatcher` test helper that sets the ready channel and blocks:
  - Why: centralizes watcher startup synchronization across all three watcher tests
  - Where: `internal/daemon/file_watcher_test.go:209-222`
- Replaced directory-watch sleep with poll-write loop:
  - Why: writing into a new directory and polling for events is more robust than a fixed 20ms sleep
  - Where: `internal/daemon/file_watcher_test.go:152-166`
- Removed unnecessary sleep in build ID monotonicity test:
  - Why: atomic sequence counter guarantees uniqueness without time separation
  - Where: `internal/build/esbuild_builder_test.go:198`

## Risk and Mitigation
- Risk: `ready` channel adds a field to the production struct.
- Mitigation: field is unexported and nil by default — `close(nil)` is never reached because the `if w.ready != nil` guard protects it. Zero cost in production.
- Risk: poll-write in directory test could produce extra events.
- Mitigation: test returns immediately when the target path appears, ignoring intermediate events.

## Verification
- Commands run:
  - `go test -race -count=1 ./internal/daemon/ ./internal/build/`
  - `make fmt && make lint && make test && make build`
- All tests pass under race detector.

## Teach-back (engineering lessons)
- Design lesson: readiness signals (channels, sync primitives) are always more reliable than fixed-duration sleeps. The only legitimate sleep in async tests is when you're testing timing behavior itself (e.g., debounce coalescing).
- Testing lesson: when testing filesystem watchers, poll-write loops handle the inherent race between "watcher registers directory" and "write file in directory" without coupling to implementation timing.

## Next Step
- Thread request context through store calls to add cancellation path for SQLite operations.

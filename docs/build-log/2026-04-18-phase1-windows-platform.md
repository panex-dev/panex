# Phase 1 Windows platform fixes — strict-green CI

**Status:** PR pending (extends `2026-04-18-phase1-ci-green.md`)
**Date:** 2026-04-18

## Problem

After the cross-platform fixes landed in `2026-04-18-phase1-ci-green.md`, four Windows-only test failures remained, blocking strict-green merge of PR #141:

1. `TestPathAccessors` (`internal/fsmodel`) — 13 subtests compared accessor output against literal forward-slash paths (`/project/.panex/state.json`). Windows produces `\project\.panex\state.json`.
2. `TestAcquireRelease` and `TestDoubleAcquire` (`internal/lock`) — the `Manager.isAlive` liveness probe uses `process.Signal(syscall.Signal(0))`. On Windows, `os.Process.Signal` only accepts `os.Kill` and `os.Interrupt`; signal 0 always errors, so every newly-acquired lock was reported as held by a dead process and immediately removed as stale.
3. `TestFileWatcherSkipsInfrastructureDirs` (`internal/daemon`) — Windows `ReadDirectoryChangesW` reports a Create event on the parent infrastructure directory itself when one of its children changes, even though the directory was not added to the watcher. The test received an event with path `.git`.
4. `TestTypeScriptProtocolParity` (`internal/protocol`) — git on Windows checks files out with CRLF line endings by default. The regex `^export const PROTOCOL_VERSION = (\d+);$` (multiline mode) fails because Go's `$` in `(?m)` matches before `\n`, leaving `\r` in front of `;`.

## Approach

- **`internal/fsmodel/fsmodel_test.go`** — wrap every expected path string in `filepath.FromSlash(...)`. The accessors themselves already use `filepath.Join`, so the production code is correct; only the test fixture was platform-coupled.

- **`internal/lock/lock.go` → `process_unix.go` / `process_windows.go`** — replaced the `Manager.isAlive` body with a delegation to a free function `isProcessAlive(pid int) bool` whose implementation is build-tagged:
  - `process_unix.go` (`//go:build !windows`) keeps the existing `os.FindProcess` + `Signal(0)` probe.
  - `process_windows.go` (`//go:build windows`) opens the process with `syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION=0x1000)` and reads `syscall.GetExitCodeProcess`. The process is alive iff the exit code is `STILL_ACTIVE = 259`.

  This is a real bug, not a test artifact — the broken liveness check would cause every Windows session to re-acquire its own lock as stale on every check.

- **`internal/daemon/file_watcher.go`** — `normalizePath` now also rejects events whose first path segment is an infrastructure dir. The fix lives in normalization rather than in `isInfrastructureDir` because `addDirectoryTree` already correctly skips those subtrees on Linux/macOS — the Windows backend just emits one extra parent-directory event that needs to be discarded.

- **`internal/protocol/parity_test.go`** — `loadSharedProtocolSource` now strips `\r\n` → `\n` before returning the source. This is purely a test concern (parsing TS source for cross-language drift checks); production code never reads this file.

## Risk and Mitigation

- **Risk (Windows liveness):** `OpenProcess` requires the calling process to have permission. Under CI the test process opens its own PID, which is always permitted.
- **Mitigation:** `isProcessAlive` returns `false` on any `OpenProcess` error, which is the safe default — it treats unknown processes as dead, matching the Unix branch's behavior when `FindProcess` fails.
- **Risk (file_watcher filter):** A legitimate file at `node_modules.txt` or `.gitkeep` at root could be wrongly filtered.
- **Mitigation:** Filter checks only the *first path segment* — files at root with infrastructure-style names are not filtered because they have no segment separator. Only entries *under* `.git/`, `node_modules/`, etc. are dropped, which matches `isInfrastructureDir`'s semantics in `addDirectoryTree`.

## Verification

- `make fmt` — clean
- `make lint` — clean
- `go test -race -count=1 ./...` — 25/25 packages pass on linux/amd64
- Windows verification deferred to CI (cannot reproduce locally).

## Next Step

Push and wait for CI matrix to be strict-green across lint, dependency-verification, go-test (ubuntu/macos/windows), and TypeScript jobs. Then merge PR #141.

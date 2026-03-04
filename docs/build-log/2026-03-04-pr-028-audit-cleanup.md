# PR28 - Audit Cleanup: Hardening Before Transport Wiring

## Metadata
- Date: 2026-03-04
- PR: 28
- Branch: `fix/pr28-audit-cleanup`
- Title: fix validation, error handling, and debuggability gaps from audit
- Commit(s): pending

## Problem
- The codebase audit (`audit-1.md`) identified validation gaps, silent error paths, and missing error context that compound as features land.
- No guard prevented `source_dir == out_dir`, risking esbuild overwriting source files.
- `readLoop` connection errors were silently swallowed, making WebSocket debugging impossible.
- `DecodePayload` returned raw msgpack errors with no call-site context.
- `collectMessages` sorted esbuild errors alphabetically, destroying source-order diagnostics.
- File watcher leaked a timer on the error path.

## Approach (with file+line citations)
- Added SourceDir == OutDir guard in esbuild builder after path resolution:
  - Why: prevents esbuild from overwriting source files with bundle output
  - Where: `internal/build/esbuild_builder.go:55-57`
- Added SourceDir == OutDir validation in config.Validate():
  - Why: catches misconfiguration at load time before builder is constructed
  - Where: `internal/config/config.go:92-94`
- Logged non-normal-close errors in readLoop before returning:
  - Why: connection drops, protocol errors, and unexpected closes were invisible
  - Where: `internal/daemon/websocket_server.go:279-281`
- Stopped debounce timer on file watcher error path:
  - Why: timer goroutine leaked when watcher returned early on error
  - Where: `internal/daemon/file_watcher.go:128-130`
- Wrapped DecodePayload marshal/unmarshal errors with context:
  - Why: raw msgpack errors gave no indication which decode phase failed
  - Where: `internal/protocol/codec.go:20-27`
- Removed alphabetical sort from collectMessages:
  - Why: esbuild returns errors in source order, which is more useful for debugging
  - Where: `internal/build/esbuild_builder.go:172` (line removed)
- Added test cases:
  - Where: `internal/build/esbuild_builder_test.go:43-48` (source equals output directory)
  - Where: `internal/config/config_test.go:159-172` (source_dir equals out_dir)

## Risk and Mitigation
- Risk: readLoop logging could be noisy for expected disconnects.
- Mitigation: only logs when error is NOT `NormalClosure` or `GoingAway` (`internal/daemon/websocket_server.go:279`).
- Risk: SourceDir == OutDir check could reject valid relative-path configs that resolve differently.
- Mitigation: config check uses raw string equality (catches exact matches); builder check uses resolved absolute paths (catches symlink/relative equivalence).

## Verification
- Commands run:
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Additional checks:
  - `go test ./internal/build -run TestNewEsbuildBuilderValidation` — new "source equals output directory" case passes
  - `go test ./internal/config -run TestLoadValidationFailures` — new "source_dir equals out_dir" case passes

## Teach-back (engineering lessons)
- Design lesson: validation belongs at every layer boundary — config rejects bad input early, builder rejects it again with resolved paths. Defense in depth costs one line per layer.
- Testing lesson: audit findings map directly to test cases. Each fix gets a test that would have caught the original issue.
- Team workflow lesson: periodic audits produce a concrete punch list. Small surgical fixes in a single PR prevent audit findings from becoming permanent tech debt.

## Next Step
- Implement transport wiring so inspector storage simulation operations call daemon mutation APIs via WebSocket, driving `storage.diff` from real mutation traffic.

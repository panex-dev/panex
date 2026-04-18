# Phase 1 file watcher — Windows new-directory race

**Status:** PR pending
**Date:** 2026-04-18

## What

`internal/daemon/file_watcher.go`: when fsnotify reports the creation of a
new subdirectory and we add it to the watcher mid-loop, we now also walk
the new directory once and queue any files already present as pending
change entries. Infrastructure directories (`.git`, `node_modules`, `.panex`,
…) are skipped during this walk, matching the behaviour of `addDirectoryTree`.

## Why

After PR #141 merged to `main`, `go-test (windows-latest)` failed on
`TestFileWatcherWatchesNewDirectories`:

```
file_watcher_test.go:164: timed out waiting for nested file event
```

The test does `MkdirAll(root/nested)` and immediately polls
`WriteFile(root/nested/entry.js)` for two seconds, expecting the watcher
to surface `nested/entry.js`.

On Linux/macOS the order of events on the parent watch — `Create(nested)`
followed by either `Create(entry.js)` (first poll) or `Write(entry.js)`
(subsequent polls) on the freshly-added child watch — is reliable enough
that one of the polls always lands. On Windows the `ReadDirectoryChangesW`
backend has a race window between the directory's creation and the
`Add(nested)` call where file events inside `nested` are dropped entirely;
once the file already exists, subsequent `os.WriteFile` calls produce only
modify events that fsnotify on Windows does not always surface for a
recently-attached watch. The test then times out.

This is a real production gap, not just a test artefact: any user editing
files inside a directory they just created (e.g. `npm init` scaffolding,
`mkdir src/feature && touch src/feature/index.ts` from an IDE) would have
the initial files silently miss the rebuild trigger on Windows until the
next event. Synthesizing the children at watch-attach time closes the gap
without changing the public API.

## Impact

- `TestFileWatcherWatchesNewDirectories` becomes deterministic on Windows.
- `TestFileWatcherSkipsNewInfrastructureDirs` continues to pass: the
  synthesize walk skips infra dirs, and `normalizePath` already filters
  any infra-prefixed path that slips through.
- No new public surface; no behavioral change on Linux/macOS where the
  pre-existing children set is empty (the test creates the dir then
  writes into it, but the `Create(nested)` event is delivered before
  `entry.js` exists, so the synthesize walk finds nothing and the
  pre-existing event flow is unchanged).

## Quality

- `make fmt` — clean
- `make lint` — clean
- `go test -race -count=1 ./internal/daemon/...` — pass on linux/amd64
- `go test -race -count=1 ./...` — pending full run
- Windows verification deferred to CI.

# Phase 1 file watcher — Windows new-directory race

**Status:** PR pending
**Date:** 2026-04-18

## What

`internal/daemon/file_watcher.go`: the watcher now runs an always-on
500 ms `treeSyncTicker` in its select loop. Each tick:

1. Walks `w.root` (skipping infrastructure dirs) and compares against
   a `watchedDirs` set. Any directory not yet registered with fsnotify
   is `Add`-ed and enrolled into `recentlyAddedDirs` with a 4-tick
   budget (~2 s of re-walk coverage).
2. Re-walks every entry in `recentlyAddedDirs` via
   `synthesizeExistingChildren`, queuing pending entries for any files
   present. Budget decrements each tick; entries drain out of the map
   when it reaches zero.

On `Create` events we still attach the child watch and synthesize
eagerly — that path stays for Linux/macOS where fsnotify reliably
delivers the event, so we don't wait up to one tick to react. The
always-on ticker is the safety net for Windows, where fsnotify can
drop the `Create(subdir)` event entirely.

Prior attempts in this branch:

- `f0666e8` — single synthesize walk on the `Create` event. Closed the
  "file already on disk when event arrives" gap but missed the case
  where the file is written *after* our walk but the modify event on
  the freshly-attached watch is dropped.
- `7938e04` — 100 ms `rewalkTicker` triggered by the `Create` event.
  Closed the "dropped modify" gap on platforms that deliver the Create
  event, but on Windows the Create event itself can be missing, so the
  ticker never started and nothing ever detected the new dir.

This change replaces the event-triggered rewalk with an unconditional
poll so detection no longer depends on fsnotify delivering the Create
event at all.

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

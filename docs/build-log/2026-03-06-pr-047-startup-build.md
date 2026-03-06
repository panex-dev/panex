# PR Build Log

## Metadata
- Date: 2026-03-06
- PR: 47
- Branch: `feat/pr47-startup-build`
- Title: trigger an initial build when the daemon starts
- Commit(s):
  - `feat(cmd): trigger startup build before watch events`

## Problem
- The daemon only built after receiving a file watcher event.
- On a cold start, users could launch `panex dev` and still have no output artifacts until they edited a file, which breaks the first-run development loop.

## Approach (with file+line citations)
- Change 1:
  - Why: run one build immediately before entering steady-state watch processing so startup produces artifacts and broadcasts.
  - Where: `cmd/panex/main.go`
- Change 2:
  - Why: distinguish startup-triggered reloads from change-triggered reloads and keep the build loop safe if the watcher channel closes.
  - Where: `cmd/panex/main.go`
- Change 3:
  - Why: update the build-loop tests to assert startup `build.complete` and `command.reload` behavior before normal file-change events.
  - Where: `cmd/panex/main_test.go`

## Risk and Mitigation
- Risk: a startup build changes the ordering of the first broadcasts observed by connected clients.
- Mitigation: tests now assert the exact startup ordering and reload reason, and the agent path remains compatible because it only reacts to `command.reload`.

## Verification
- Commands run:
  - `gofmt -w cmd/panex/main.go cmd/panex/main_test.go`
  - `GOCACHE=/tmp/go-build go test ./cmd/panex -count=1`
  - `GOCACHE=/tmp/go-build go test -race -count=1 ./cmd/panex`
  - `GOCACHE=/tmp/go-build go test -race -count=1 ./...`
  - `GOCACHE=/tmp/go-build go build ./cmd/panex`
- Additional checks:
  - `pnpm --dir shared/protocol run check`
  - `pnpm --dir shared/protocol run test`
  - `pnpm --dir agent run check`
  - `pnpm --dir agent run test`
  - `pnpm --dir inspector run check`
  - `pnpm --dir inspector run test`
  - `pnpm --dir shared/chrome-sim run check`
  - `pnpm --dir shared/chrome-sim run test`

## Teach-back (engineering lessons)
- Design lesson: watch-driven systems still need explicit bootstrap behavior; waiting for the first external event is not a reliable startup contract.
- Testing lesson: once startup work exists, tests need to assert event ordering explicitly instead of assuming the first broadcast always comes from a file change.
- Team workflow lesson: isolating each fix in its own worktree branch makes it practical to run full verification without contaminating `main`.

## Next Step
- Prevent overlapping `source_dir` and `out_dir` paths so the new startup build cannot trigger self-generated rebuild loops in common `dist/` layouts.

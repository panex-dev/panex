# PR9 - Daemon Reload Command Emission

## Metadata
- Date: 2026-03-01
- PR: 9
- Branch: `feat/extension-reload`
- Title: emit command.reload after successful build.complete
- Commit(s): pending

## Problem
- The daemon published build completion events, but never issued reload commands.
- This left the save-to-reload loop incomplete even though the Dev Agent runtime command handler already existed.

## Approach (with file+line citations)
- Extended build loop orchestration to emit `command.reload` only on successful builds and include build correlation fields:
  - `cmd/panex/main.go:232-249`
- Kept reliability semantics consistent by logging reload broadcast errors without terminating daemon runtime:
  - `cmd/panex/main.go:246-248`
- Expanded CLI build-loop tests to verify strict success/failure behavior:
  - success path now requires `build.complete` then `command.reload` in `cmd/panex/main_test.go:237-305`
  - failure path asserts no reload command in `cmd/panex/main_test.go:307-357`
  - added helper for targeted message-count assertions in `cmd/panex/main_test.go:476-488`
- Captured architectural policy for reload emission ordering:
  - `docs/adr/008-reload-command-emission.md:1-26`

## Risk and Mitigation
- Risk: command storms if reload is emitted on failed or partial builds.
- Mitigation: gate reload on `result.Success` and carry reason/build id for consumer-side filtering (`cmd/panex/main.go:232-244`).
- Risk: one failed broadcast could interrupt local dev feedback.
- Mitigation: non-fatal logging path for both build and reload broadcast failures (`cmd/panex/main.go:227-230`, `cmd/panex/main.go:246-248`).

## Verification
- Commands:
  - `go test ./cmd/panex -run 'TestRunBuildLoop'`
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Expected:
  - build loop tests validate ordering and no-reload-on-failure behavior
  - full repository quality gates stay green

## Teach-back (engineering lessons)
- Command emission should encode outcome policy, not just transport mechanics.
- Ordering contracts (`event` before `command`) are easiest to enforce at one orchestration boundary.
- Tests should validate both positive behavior and "must not happen" behavior for control messages.

## Next Step
- Add end-to-end integration coverage between daemon and Dev Agent WebSocket sessions to validate real wire payloads in one test loop.

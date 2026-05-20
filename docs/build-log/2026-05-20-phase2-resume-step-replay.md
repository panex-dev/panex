# Phase 2 plan/apply — resume step replay

**Status:** PR pending
**Date:** 2026-05-20

## Problem

The Phase 2 deferred-feature inventory still listed `resume` step replay as incomplete. The CLI and MCP `resume_run` surfaces loaded the run ledger, transitioned the run to `running`, and then marked it `succeeded` without replaying the failed or incomplete plan steps. That made `resume` look successful while leaving the intended mutation unapplied.

## Approach

- Added a shared `plan.Resume` replay path that accepts the durable run plus saved plan, skips already-succeeded apply steps, and replays the first failed or incomplete action onward.
- Treat runs that executed successful rollback steps as fully reverted and replay all planned actions from the start.
- Allowed resumable failed runs to transition back to `running`, and clear stale completion timestamps when a terminal run re-enters a non-terminal state.
- Routed both CLI `panex resume` and MCP `resume_run` through the shared replay path.
- Updated apply orchestration to refresh `.panex/state.json` with the latest apply run ID so `panex resume` can default to the most recent apply run.
- Updated the deferred-feature inventory to mark the Phase 2 resume replay gap complete.

## Risk and mitigation

- Risk: replaying already-applied work could duplicate mutations.
- Mitigation: resume skips the prefix of succeeded apply steps and only replays later failed or missing steps, unless rollback evidence shows prior work was explicitly undone.

- Risk: CLI and MCP resume behavior could drift.
- Mitigation: both surfaces now delegate to the same `plan.Resume` implementation.

- Risk: failed terminal runs could retain stale completion metadata after resuming.
- Mitigation: ledger transitions now clear `completed_at` when moving back to a non-terminal state.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./internal/plan -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/cli -run 'TestCmdResume|TestCmdApply|TestFullWorkflow' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/mcp -run TestToolResume -count=1`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` failed before linting with Go VCS stamping: `error obtaining VCS status: exit status 128`.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make build` failed before compiling with Go VCS stamping: `error obtaining VCS status: exit status 128`.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the remaining Phase 2 deferred-feature inventory with the MCP `rollback_changes` tool or move to the next Phase 3 item if that tool has already been superseded by the automatic apply rollback path.

# Phase 2 MCP surface — rollback failed apply changes

**Status:** PR pending
**Date:** 2026-05-20

## Problem

The deferred-feature inventory still listed `rollback_changes` as a Phase 2 MCP gap. The plan/apply layer had automatic best-effort rollback during apply failures, but MCP clients had no explicit tool to roll back a failed or interrupted apply run from durable ledger evidence.

## Approach

- Added `plan.Rollback`, a shared rollback operation that reads a durable run and plan, identifies successful apply steps, and rolls them back in reverse plan order.
- Skips rollback work already recorded as successful so retrying `rollback_changes` is idempotent instead of destructive.
- Rejects non-apply runs and succeeded apply runs so MCP clients cannot accidentally roll back a completed successful mutation.
- Exposes MCP `rollback_changes` with optional `run_id`, defaulting to `.panex/state.json`'s latest run when omitted.
- Records rollback steps back into the run ledger and leaves the original apply run terminally failed, preserving the truth that the original operation did not complete successfully.
- Raised the TypeScript config evaluation timeout from 15s to 30s after Windows race CI exceeded the previous bound during the first Node evaluation. The timeout remains bounded and the timeout-specific test continues to use a 10ms injected deadline.
- Updated the deferred-feature inventory to remove the completed Phase 2 `rollback_changes` gap.

## Risk and mitigation

- Risk: rollback could run twice and remove files that were recreated by another actor.
- Mitigation: the rollback target selector counts already successful rollback steps and skips those already reversed.

- Risk: rollback could be applied to the wrong kind of run.
- Mitigation: `plan.Rollback` rejects any run whose operation is not `apply`, and it rejects succeeded apply runs.

- Risk: manual rollback could conflict with active mutation work.
- Mitigation: `plan.Rollback` acquires the project mutation lock before executing action rollback handlers.

- Risk: increasing the TypeScript config evaluation timeout could hide a truly hung config longer.
- Mitigation: evaluation remains deadline-bound, and the timeout path is covered by a stubbed 10ms test.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./internal/plan -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/mcp -run 'TestToolsList|TestToolRollbackChanges' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/ledger -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/configloader -count=1`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` failed before linting with Go VCS stamping: `error obtaining VCS status: exit status 128`.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build` failed before compiling with Go VCS stamping: `error obtaining VCS status: exit status 128`.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`

## Next Step

Continue the deferred MCP surface with `query_run_history` and `configure_project`, both currently tracked for Phase 3.

# Phase 3 CLI Interactive Flag

**Status:** implemented
**Date:** 2026-05-20

## What

Add the global CLI `--interactive` and `--yes` flags, make JSON the default non-interactive output mode, and preserve human-readable output behind the explicit interactive mode.

## Why

The CLI spec requires Panex to be programmatic by default while still offering an opt-in human mode. Before this change, commands accepted `--json` but human output remained implicit for several command surfaces, and the deferred `--interactive` flag was not parsed.

## Approach

- Updated global usage text to advertise `--json|--interactive` and `--yes` consistently across command surfaces.
- Added global parsing for `--interactive` and `--yes`, with `--json` and `--interactive` rejected as mutually exclusive.
- Made default output mode JSON unless `--interactive` is passed.
- Updated command tests so human-output assertions opt into `--interactive`, and added coverage for default JSON, mutual exclusion, and `--yes` acceptance.
- Raised the TypeScript config evaluator timeout from 15s to 30s after Windows CI proved the existing deadline could expire before Node/esbuild startup completed.
- Removed the deferred CLI flag gap from the spec gap inventory.

## Risk and Mitigation

- Risk: existing manual workflows may expect human output without flags. Mitigation: human output is still available through `--interactive`, and JSON-by-default matches the documented agent-first CLI contract.
- Risk: `--yes` could imply prompts exist today. Mitigation: the flag is accepted as a no-op until interactive prompts are introduced, which preserves non-blocking behavior.
- Risk: increasing the config evaluator timeout could hide a hung evaluator longer. Mitigation: the deadline still bounds execution, and the test-only timeout override keeps timeout-path coverage fast.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./cmd/panex -count=1`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build go test ./internal/configloader -run TestLoad_TypeScriptConfig -count=1`
- `GOCACHE=/tmp/go-build go test ./cmd/panex ./internal/configloader -count=1`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` (failed in the linked worktree because Go could not obtain VCS status; rerun with VCS stamping disabled)
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make build` (failed in the linked worktree because Go could not obtain VCS status; rerun with VCS stamping disabled)
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Teach-Back

Output mode is part of the CLI contract, not presentation polish. Keeping agent-safe JSON as the default prevents accidental prompt or formatting drift as more commands are added.

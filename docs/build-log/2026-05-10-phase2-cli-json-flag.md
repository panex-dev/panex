# Phase 2 CLI surface — add global `--json`

**Status:** PR pending
**Date:** 2026-05-10

## Problem

The deferred-feature inventory still listed the global `--json` flag as a Phase 2 gap. The command boundary in `cmd/panex` was split:

- the core commands routed through `internal/cli` already emitted the stable JSON envelope,
- but several top-level command surfaces still printed plain text directly from `cmd/panex`:
  - `version`
  - `help`
  - `init`
  - `dev`
  - `doctor`
  - `paths`
- top-level parser errors and subcommand flag-validation errors were also text-only.

That left the spec’s “force JSON output mode” contract unimplemented even though most of the repo had already standardized on JSON-first responses.

## Approach

- Added a real global `--json` flag in `cmd/panex/main.go` and updated the usage text so the command surface advertises the mode explicitly.
- Kept the existing JSON behavior for core commands that already route through `internal/cli`.
- Added shared JSON envelope helpers in `cmd/panex/json_output.go` so the remaining top-level text commands can emit the same `status` / `command` / `summary` / `data` shape.
- Switched the remaining text surfaces to support JSON mode:
  - `version` and `help`
  - `init`
  - `dev` startup metadata
  - `doctor`
  - `paths`
- Routed top-level/global parsing failures and subcommand flag-validation failures (`add-target`, `apply`, `package`, `report`, `resume`, `dev`, `init`, `paths`) through JSON envelopes when `--json` is present.
- Updated the Phase 2 deferred-feature inventory to remove the completed `--json` gap.

## Risk and mitigation

- Risk: JSON mode could diverge from the command surfaces that already emit JSON via `internal/cli`.
- Mitigation: the new helpers reuse the same `internal/cli.Output` envelope rather than inventing a second command-response shape.

- Risk: `dev` could report success before the runtime is actually configured.
- Mitigation: startup output emission was moved behind the runtime setup path in `startDevServer`, so JSON startup metadata is written only after websocket server, builders, and watchers are configured successfully.

- Risk: error-mode changes could break existing human-readable flows.
- Mitigation: the default behavior remains text-first for the same commands; JSON output only activates when `--json` is explicitly present.

## Verification

- `GOCACHE=/tmp/go-build go test ./cmd/panex -count=1`
- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the remaining Phase 2 deferred-feature inventory with the next real behavior gap, most likely `resume` step replay or MCP `rollback_changes`, now that the global CLI output contract matches the repo’s JSON-first command model.

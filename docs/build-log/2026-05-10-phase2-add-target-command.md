# Phase 2 CLI + MCP surface — add `add-target` / `add_target`

**Status:** PR pending
**Date:** 2026-05-10

## Problem

The deferred Phase 2 inventory still listed `add-target` and the matching MCP `add_target` tool as missing. Projects could inspect, init, plan, apply, verify, and package, but there was no narrow command for expanding the authored target set after the initial graph existed.

That gap was larger than a single config write. Many projects still do not have an authored `panex.config.json`, only an inferred or previously written graph under `.panex/`, so a usable `add-target` flow also needed to bootstrap authored config from existing project state instead of failing on missing config.

## Approach

- Added shared target-add logic in `internal/cli/cli.go` that:
  - validates target names against the currently known platform set,
  - bootstraps `panex.config.json` from the existing graph when no authored config exists yet,
  - enables the requested target in config,
  - expands `panex.policy.toml` deterministically,
  - rebuilds `.panex/project.graph.json` from the updated config plus fresh inspection.
- Exposed that logic through the JSON CLI command `CmdAddTarget(...)` and the top-level `panex add-target <target>` route in `cmd/panex/main.go` and `cmd/panex/core.go`.
- Added the MCP `add_target` tool in `internal/mcp/mcp.go`, reusing the same `internal/cli` target-add path instead of duplicating mutation logic.
- Sorted enabled config targets in `internal/graph/builder.go` so graph target ordering stays deterministic after reading `panex.config.json`.
- Fixed the manual policy writer in `internal/cli/cli.go` to emit valid TOML string arrays with commas, because the new target-sync path exposed that the prior array rendering was not parseable once more than one allowed target existed.
- Added focused regression coverage in:
  - `internal/cli/cli_test.go`
  - `internal/graph/builder_test.go`
  - `internal/mcp/mcp_test.go`
  - `cmd/panex/main_test.go`

## Risk and mitigation

- Risk: bootstrapping `panex.config.json` from an inferred or graph-derived project could accidentally discard authored state.
- Mitigation: the bootstrap path only runs when no authored config exists. Existing config files are loaded and updated in place.

- Risk: allowing known-but-unimplemented targets such as `firefox` could make the graph claim support that the runtime cannot actually package yet.
- Mitigation: the rebuilt graph keeps those targets in `targets_requested` but resolves only adapters that are actually registered today, and the command returns a warning when a target is not yet resolved.

- Risk: changing target ordering could alter graph hashes unexpectedly.
- Mitigation: target ordering is now explicitly sorted when converting loaded config into graph input, which removes map-iteration nondeterminism instead of introducing new randomness.

## Verification

- `go test ./internal/cli ./internal/graph ./internal/mcp ./cmd/panex`

## Next Step

Continue the remaining deferred-feature inventory with another narrow Phase 2 CLI/MCP or plan/apply slice, now that post-init target expansion is no longer blocked on handwritten config edits.

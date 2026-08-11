# README Current Panex Rewrite

## Metadata
- Date: 2026-08-11
- Scope: product documentation
- Branch: `docs/readme-new-panex`

## Problem
- The root README still framed Panex primarily as the older Chrome extension build/watch runtime.
- That under-described the newer Panex surface: project inspection, graph/state management, plan/apply, target-aware manifest generation, verification, packaging, run reports, global JSON output, global `--cwd`, and the MCP stdio server.
- The README also blurred two active configuration modes: `panex.toml` for the Chrome dev loop and `panex.config.ts` / `panex.config.json` for the project automation workflow.

## Approach
- Rewrote the root README product overview so it presents Panex as an agent-oriented browser-extension engineering runtime.
- Documented the actual top-level CLI surface, including `add-target`, `inspect`, `plan`, `apply`, `test`, `verify`, `package`, `report`, `resume`, `doctor`, `paths`, `mcp`, `--cwd`, and `--json`.
- Split first-run guidance into two paths:
  - Chrome dev loop via `panex init`, `panex dev --open`, `panex.toml`, and `.panex/dist`.
  - Project automation via inspect/add-target/plan/apply/verify/package/report and `.panex/` state.
- Documented the MCP tools/resources and the current Chrome runtime simulator limits.
- Updated the build status tracker so repo memory records that the product README now reflects the current Panex surface.

## Risk and Mitigation
- Risk: the README could overstate unfinished Phase 2 behavior.
- Mitigation: the rewrite keeps the status as early development, calls Chrome the implemented target adapter today, notes that runtime/storage isolation remains partial for multi-extension dev-loop use, and avoids claiming full resume replay or MCP rollback support.
- Risk: users could confuse `panex.toml` with `panex.config.json` / `panex.config.ts`.
- Mitigation: the README explicitly names which workflow uses each config file.

## Verification
- Commands run:
  - `git diff --check`
  - `make check` - initially failed because the fresh worktree had no `node_modules` and `tsc` was unavailable.
  - `pnpm install`
  - `make check`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Read the README against `cmd/panex/main.go`, `cmd/panex/init.go`, `internal/config/config.go`, `internal/configloader/configloader.go`, `internal/cli/cli.go`, and `internal/mcp/mcp.go`.

## Teach-back
- Product docs need to track the executable command surface, not just the original product idea.
- When a repo has transitional workflows, naming the configuration boundary directly prevents users and agents from combining incompatible setup steps.

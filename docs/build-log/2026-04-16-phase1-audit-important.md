# Phase 1 Audit — Important Design Issues

**Status:** in review
**Date:** 2026-04-16

## What

Fix 4 important design issues found during the Phase 1 audit that would compound into larger problems during Phase 2.

## Why

These are design-level issues that don't produce incorrect behavior today but would create friction or risk during multi-target, multi-adapter Phase 2 work: duplicate config-to-graph type mapping done manually in two places, missing run subdirectory creation, misleading YAML struct tags on a TOML-only format, and no integration test covering the full command workflow.

## Fixes

| # | Issue | File(s) | Fix |
|---|---|---|---|
| 8 | Duplicate configloader→graph manual mapping | `graph/builder.go`, `cli/cli.go`, `mcp/mcp.go` | Added `graph.ProjectConfigFromLoaded()` conversion function, replaced 20-line manual mapping in both cli and mcp |
| 9 | No on-demand run subdirectory creation | `fsmodel/fsmodel.go` | Added `EnsureRunDirs(runID, targets)` to create run/generated/manifests and trace dirs |
| 10 | Policy structs carry dead `yaml:` tags | `policy/policy.go`, `fsmodel/fsmodel.go`, `inspector/inspector.go`, `fsmodel/fsmodel_test.go` | Removed all `yaml:` tags, renamed constant and references from `panex.policy.yaml` to `panex.policy.toml` |
| 11 | No integration test for full workflow | `cli/cli_test.go` | Added `TestFullWorkflow_Inspect_Plan_Apply_Verify_Package` covering init→plan→apply→verify→package→report |

## Bonus

- Fixed pre-existing `nilerr` lint violation in `inspector/inspector.go` (WalkDir callback returned nil on error)

## Impact

- 10 files changed across 7 packages
- All 24 packages pass `go test -race`
- `make fmt` — clean
- `go build` — binary builds

## Quality

- `go test -race -count=1 ./internal/... ./cmd/panex/...` — 24/24 pass
- New tests: `TestProjectConfigFromLoaded`, `TestProjectConfigFromLoaded_Nil`, `TestEnsureRunDirs`, `TestFullWorkflow_Inspect_Plan_Apply_Verify_Package`

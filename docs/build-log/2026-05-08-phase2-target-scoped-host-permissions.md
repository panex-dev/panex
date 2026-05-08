# Phase 2 manifest compiler — target-scoped host permissions

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The capability matrix still stored host permissions as one shared slice for all targets. That matched the Phase 1 single-target assumption, but it blocked the Phase 2 manifest compiler from emitting different `host_permissions` sets per target in multi-target projects.

## Approach

- Added `HostPermsByTarget` to `capability.TargetMatrix` and `HostPermissionsByTarget` to `capability.CompilerInput`.
- Kept the existing aggregated `HostPerms` slice for verification and compatibility, but derive it from the union of all target-specific host permissions.
- Updated `manifest.hostPermsFromMatrix(...)` to read `matrix.HostPermissionsForTarget(tgt)` instead of the old matrix-wide slice.
- Added tests covering:
  - global host permissions still fanning out to a single target,
  - per-target host permission overrides in the capability matrix,
  - manifest compilation producing different `HostPermissions` for different targets.

## Risk and mitigation

- Risk: existing callers that only know about the old shared host-permission field could regress.
- Mitigation: the old `HostPerms` aggregate remains populated, and `HostPermissionsForTarget(...)` falls back to that aggregate when no target-specific map is present.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/capability/...`
- `GOCACHE=/tmp/go-build go test ./internal/manifest/...`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Use the same target-scoped pattern for the next multi-target compiler/runtime slices, starting with runtime identity collection and then any remaining per-target manifest data that still assumes a single shared value.

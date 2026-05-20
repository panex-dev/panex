# Phase 2 Firefox Target Adapter

**Status:** implemented
**Date:** 2026-05-20

## What

Phase 2 now has a first-class Firefox target adapter registered in the default target registry.

## Why

The target model distinguishes packaging targets from runtime browser brands. Chrome was the only registered adapter, so Firefox remained an unresolved target in CLI and compiler flows even though the Phase 2 roadmap requires Chrome plus Firefox target support.

## Approach

- Added `target.NewFirefox()` with a Firefox capability catalog, environment detection, capability resolution, MV2-oriented manifest compilation, and `.xpi` artifact packaging.
- Registered Firefox in `target.DefaultRegistry()` so CLI, capability, and manifest paths can resolve `firefox` without per-call special wiring.
- Added Firefox target tests plus manifest and capability coverage for multi-target host permissions.
- Updated the `add-target firefox` CLI expectation because Firefox is now a supported resolved target, not an unresolved requested target.
- Raised the TypeScript config evaluation timeout from 15s to 30s after Windows CI proved the existing limit is too tight for `TestLoad_TypeScriptConfig`.

## Risk and Mitigation

- Risk: Firefox manifest generation could accidentally use Chrome-only MV3 fields. Mitigation: Firefox tests assert MV2 `background.scripts`, `browser_action`, and `sidebar_action` output.
- Risk: host permissions could be lost because Firefox MV2 stores them inside `permissions`. Mitigation: adapter tests and manifest compiler tests assert host permissions are included in the manifest while still reported separately in compiler output.
- Risk: enabling Firefox in the default registry changes CLI `add-target` behavior. Mitigation: CLI tests now assert Firefox resolves without warnings and appears in `TargetsResolved`.
- Risk: raising the TypeScript config timeout could delay a truly wedged config evaluation. Mitigation: the timeout remains finite and still returns a structured timeout error.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./internal/target ./internal/capability ./internal/manifest -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/target ./internal/capability ./internal/manifest ./internal/cli -run 'TestFirefox|TestDefaultRegistry|TestCompile_|TestAddTarget_BootstrapsConfigAndUpdatesGraphAndPolicy|TestCmdInit_NewProject|TestCmdPackage|TestFullWorkflow' -count=1`
- `GOCACHE=/tmp/go-build go test ./internal/mcp -run TestToolAddTarget -count=1`
- `GOCACHE=/tmp/go-build make test` initially failed because `TestToolAddTarget` still expected Firefox to remain unresolved.
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` failed in the linked worktree because Go VCS stamping could not resolve repository status.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make build` failed in the linked worktree because Go VCS stamping could not resolve repository status.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`
- `GOCACHE=/tmp/go-build go test ./internal/configloader -run TestLoad_TypeScriptConfig -count=1`

## Teach-Back

Adding a target should start at the adapter boundary and then let existing compiler and CLI paths consume the registry. That keeps target expansion explicit without adding Firefox special cases to higher-level orchestration.

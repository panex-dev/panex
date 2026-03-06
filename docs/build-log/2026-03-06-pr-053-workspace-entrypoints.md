# PR53 - Workspace Package Entrypoints for Cross-Package TypeScript Imports

## Metadata
- Date: 2026-03-06
- PR: 53
- Branch: `feat/pr53-workspace-entrypoints`
- Title: move cross-package TypeScript imports onto workspace package entrypoints
- Commit(s): pending

## Problem
- The repo had a root `pnpm` workspace and shared tsconfig presets, but the TypeScript packages still imported one another through sibling `src/` paths.
- That kept package boundaries implicit, made future shared tooling riskier, and meant new surfaces such as Workbench would inherit coupling instead of real workspace contracts.

## Approach (with file+line citations)
- Change 1:
  - Why: declare internal workspace dependencies explicitly so `agent`, `inspector`, and `chrome-sim` consume `@panex/protocol` and `@panex/chrome-sim` as packages instead of sibling source trees.
  - Where: `agent/package.json:7-22`
  - Where: `inspector/package.json:7-24`
  - Where: `shared/chrome-sim/package.json:17-29`
  - Where: `pnpm-lock.yaml:11-70`
- Change 2:
  - Why: rewrite source and test imports to the workspace package entrypoints and route inspector’s `chrome-sim` build through a local wrapper that imports `@panex/chrome-sim`.
  - Where: `agent/src/background.ts:1-5`
  - Where: `agent/src/reload.ts:1-9`
  - Where: `agent/tests/protocol.test.ts:1-6`
  - Where: `agent/tests/reload.test.ts:1-7`
  - Where: `inspector/src/connection.ts:1-29`
  - Where: `inspector/src/storage.ts:1-5`
  - Where: `inspector/src/tabs/storage.ts:1-11`
  - Where: `inspector/src/timeline.ts:1-18`
  - Where: `inspector/src/chrome-sim.ts:1`
  - Where: `inspector/tests/storage.test.ts:1-14`
  - Where: `inspector/tests/timeline.test.ts:1-13`
  - Where: `inspector/scripts/build.ts:14-30`
  - Where: `inspector/scripts/preview_injection.ts:1-5`
  - Where: `shared/chrome-sim/src/runtime.ts:1-2`
  - Where: `shared/chrome-sim/src/transport.ts:1-11`
  - Where: `shared/chrome-sim/tests/runtime.test.ts:1-6`
  - Where: `shared/chrome-sim/tests/transport.test.ts:1-6`
- Change 3:
  - Why: update package docs and status tracking so contributors see the package entrypoint boundary as the supported path going forward.
  - Where: `agent/README.md:15-17`
  - Where: `inspector/README.md:15-21`
  - Where: `shared/protocol/README.md:13-19`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:52-60`
  - Where: `docs/build-log/2026-03-06-pr-053-workspace-entrypoints.md:1-72`

## Risk and Mitigation
- Risk: package entrypoint imports might resolve differently than sibling source imports in `tsc`, `tsx`, or esbuild, causing silent build-tool divergence.
- Mitigation: verification covers package `check`, `test`, and `build` flows through both package-level scripts and root `make` targets after the workspace dependency graph is updated.
- Risk: inspector’s `chrome-sim` output could become coupled to a node_modules path or fail to bundle if the build entrypoint is not explicit.
- Mitigation: the build now goes through a local wrapper source file that imports `@panex/chrome-sim`, keeping the emitted artifact on a stable local entrypoint while still honoring the package boundary.

## Verification
- Commands run:
  - `pnpm install`
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Expected:
  - All packages resolve internal workspace imports through package entrypoints rather than sibling source paths.
  - Existing test/build behavior remains unchanged after the dependency graph and imports move to workspace packages.

## Teach-back (engineering lessons)
- Design lesson: a workspace is only real when packages consume one another through declared package boundaries, not relative source paths.
- Testing lesson: import-boundary refactors must exercise the actual bundlers and runtime loaders, not just type-checks.
- Team workflow lesson: once a boundary is made explicit in code, the docs should teach only that boundary to prevent regressions.

## Next Step
- Decide whether repeated workspace script orchestration should move behind shared package-level commands now that internal imports use workspace entrypoints.

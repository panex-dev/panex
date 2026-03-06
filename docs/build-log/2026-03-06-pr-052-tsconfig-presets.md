# PR52 - Shared TypeScript Compiler Presets

## Metadata
- Date: 2026-03-06
- PR: 52
- Branch: `feat/pr52-tsconfig-presets`
- Title: extract shared TypeScript compiler presets without centralizing package build behavior
- Commit(s): pending

## Problem
- The root workspace lockfile removed duplicated dependency installation, but each TypeScript package still repeated the same compiler baseline by hand.
- Leaving that baseline duplicated would make future TypeScript upgrades and new packages like Workbench more error-prone, while centralizing package build scripts now would over-couple packages with meaningfully different runtime needs.

## Approach (with file+line citations)
- Change 1:
  - Why: define a root compiler baseline for shared strictness and module settings, plus a DOM-oriented preset for browser-facing packages.
  - Where: `tsconfig.base.json:1-10`
  - Where: `tsconfig.dom.json:1-7`
- Change 2:
  - Why: convert each package to extend the smallest appropriate preset while preserving its package-specific environment settings.
  - Where: `agent/tsconfig.json:1-8`
  - Where: `inspector/tsconfig.json:1-8`
  - Where: `shared/protocol/tsconfig.json:1-7`
  - Where: `shared/chrome-sim/tsconfig.json:1-4`
- Change 3:
  - Why: record the architectural decision that compiler policy is centralized now, while shared build/test script abstractions are intentionally deferred until package boundaries are cleaner.
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:52-59`
  - Where: `docs/build-log/2026-03-06-pr-052-tsconfig-presets.md:1-54`

## Risk and Mitigation
- Risk: a shared preset could accidentally smuggle browser or JSX assumptions into packages that should remain environment-neutral.
- Mitigation: the base preset only contains compiler invariants; browser-specific libraries stay isolated in `tsconfig.dom.json`, and each package keeps its own overrides.
- Risk: the team could overread this refactor as approval to centralize all TypeScript scripting next.
- Mitigation: the build log explicitly documents the boundary: centralize compiler defaults now, defer shared build abstractions until cross-package imports use real workspace entrypoints.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Expected:
  - All packages still type-check, test, and build after switching to shared presets.
  - Go/TypeScript root workflows remain unchanged apart from using the extracted compiler baseline.

## Teach-back (engineering lessons)
- Design lesson: centralize the stable layer first; compiler defaults are a better shared boundary than package build behavior.
- Testing lesson: config refactors still need full product verification because “just config” changes can silently alter test or build environments.
- Team workflow lesson: documenting non-goals is part of the design work; it prevents cleanup PRs from turning into accidental architecture rewrites.

## Next Step
- Move cross-package TypeScript imports onto workspace package entrypoints before considering deeper shared script/build abstractions.

# PR45 - Inspector Preview Build Hook for Chrome-Sim Entrypoint Injection

## Metadata
- Date: 2026-03-05
- PR: 45
- Branch: `feat/pr45-preview-injection-hook`
- Title: hook inspector preview build path to `injectChromeSimEntrypoint(...)`
- Commit(s): pending

## Problem
- The reusable helper `injectChromeSimEntrypoint(...)` existed in `@panex/chrome-sim`, but no renderer/plugin build path was actually calling it.
- Preview HTML generation still depended on manual script wiring, so simulator bootstrap could drift across surfaces.

## Approach (with file+line citations)
- Switched inspector build execution to a TypeScript build script (via `tsx`) so build-time code can import and call shared chrome-sim helpers:
  - `inspector/package.json:6-10`
  - `inspector/scripts/build.ts:1-52`
- Added preview HTML injection utilities that call `injectChromeSimEntrypoint(...)`, render deterministic script tags, and enforce idempotent injection:
  - `inspector/scripts/preview_injection.ts:1-75`
- Wired inspector build output to include both main bundle and bundled chrome-sim runtime, then emit `dist/index.html` with injected bootstrap metadata:
  - `inspector/scripts/build.ts:13-37`
- Added tests for injection behavior (insert, idempotent, missing-head validation, and attribute escaping):
  - `inspector/tests/preview_injection.test.ts:1-60`
- Updated inspector docs to describe injected preview output and env overrides:
  - `inspector/README.md:15-19`

## Risk and Mitigation
- Risk: bundling shared chrome-sim code from a sibling package could fail module resolution in isolated worktrees.
- Mitigation: build script pins `absWorkingDir` and `nodePaths` to inspector-local `node_modules` for deterministic resolution (`inspector/scripts/build.ts:14-22`).
- Risk: repeated build runs could duplicate injected chrome-sim script tags.
- Mitigation: injection utility exits early when `data-panex-chrome-sim` is already present (`inspector/scripts/preview_injection.ts:12-15`) and test coverage enforces this (`inspector/tests/preview_injection.test.ts:26-35`).

## Verification
- Commands run:
  - `cd inspector && pnpm install --frozen-lockfile`
  - `cd inspector && pnpm run check`
  - `cd inspector && pnpm run test`
  - `cd inspector && pnpm run build`
- Expected:
  - TypeScript checks pass.
  - All inspector tests pass, including `preview_injection.test.ts`.
  - Build emits `dist/main.js`, `dist/chrome-sim.js`, and `dist/index.html` with injected `data-panex-*` bootstrap script.

## Teach-back (engineering lessons)
- Shared bootstrap helpers only reduce drift when they are actually invoked from a build/runtime path.
- Build scripts that bridge packages should explicitly define module resolution roots to stay stable in separate worktrees.
- Injection logic should be idempotent by default because build pipelines often rerun repeatedly during watch/dev flows.

## Next Step
- Extend the same injection hook pattern to extension output HTML surfaces in the core build pipeline so simulator bootstrap is consistent beyond inspector preview builds.

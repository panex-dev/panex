# PR41 - Chrome-Sim Entrypoint Injection + Script Bootstrap Helpers

## Metadata
- Date: 2026-03-05
- PR: 41
- Branch: `feat/pr41-preview-entrypoint-injection`
- Title: add reusable entrypoint injection/bootstrap helpers for preview surfaces
- Commit(s): pending

## Problem
- `installChromeSim()` supported explicit options and URL/global fallbacks, but preview entrypoint injection had no first-class helper to attach bootstrap metadata on injected scripts.
- Without script bootstrap resolution, future preview injectors (Vite/plugin/server) would need custom glue for passing daemon URL/token/extension ID into each surface.

## Approach (with file+line citations)
- Added a dedicated bootstrap helper module in `@panex/chrome-sim`:
  - resolves bootstrap values with precedence `query params > script dataset > window globals`:
  - `shared/chrome-sim/src/bootstrap.ts:39-58`
  - entrypoint injection utility writes `data-panex-*` attributes on module script:
  - `shared/chrome-sim/src/bootstrap.ts:60-86`
  - script dataset parsing (`data-panex-chrome-sim`, `data-panex-ws`, `data-panex-token`, `data-panex-extension-id`):
  - `shared/chrome-sim/src/bootstrap.ts:88-137`
- Wired install flow to consume bootstrap helper output:
  - `shared/chrome-sim/src/index.ts:73-82`
- Exposed helper API as package export:
  - `shared/chrome-sim/package.json:7-14`
- Added tests for bootstrap resolution and script injection:
  - `shared/chrome-sim/tests/bootstrap.test.ts:6-69`
  - added install-path test for script-provided extension ID:
  - `shared/chrome-sim/tests/index.test.ts:72-114`
- Updated package README scope to include script bootstrap + injection helper:
  - `shared/chrome-sim/README.md:17-20`

## Risk and Mitigation
- Risk: multiple bootstrap channels (query/script/global) could lead to inconsistent precedence.
- Mitigation: precedence is centralized in one function and covered by deterministic tests (`shared/chrome-sim/src/bootstrap.ts:39-58`, `shared/chrome-sim/tests/bootstrap.test.ts:7-35`).
- Risk: injection helper could silently fail in non-browser contexts.
- Mitigation: helper explicitly returns `null` when no document/head is available (`shared/chrome-sim/src/bootstrap.ts:64-66`).

## Verification
- Commands run:
  - `cd shared/chrome-sim && pnpm run check`
  - `cd shared/chrome-sim && pnpm run test`
- Expected:
  - strict typecheck passes.
  - test suite passes including bootstrap and install resolution tests.

## Teach-back (engineering lessons)
- Entrypoint bootstrap concerns should be encapsulated in the shim package, not duplicated across preview runners.
- Script dataset metadata is a practical transport for injected-module configuration when URL query parameters are not ideal.
- Explicit precedence tests prevent accidental behavior drift as additional bootstrap channels are added.

## Next Step
- Use `injectChromeSimEntrypoint(...)` from the preview renderer/plugin path so each preview surface gets deterministic chrome-sim injection without ad-hoc script assembly.

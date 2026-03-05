# PR46 - Core Build Pipeline HTML Injection for Chrome-Sim

## Metadata
- Date: 2026-03-06
- PR: 46
- Branch: `feat/pr46-extension-html-injection`
- Title: extend core Go build pipeline to copy HTML surfaces and inject chrome-sim bootstrap
- Commit(s): pending

## Problem
- PR45 proved the `injectChromeSimEntrypoint(...)` flow on the inspector build path, but the shared Go builder used by `panex dev` still ignored HTML files entirely.
- That meant extension HTML surfaces would not be emitted to `outDir`, would not have script paths rewritten to bundled `.js`, and would not auto-bootstrap the simulator transport.

## Approach (with file+line citations)
- Extended `internal/build` to support optional chrome-sim injection configuration:
  - builder option/config surface at `internal/build/esbuild_builder.go:17`, `internal/build/esbuild_builder.go:32`, `internal/build/esbuild_builder.go:47`, and `internal/build/esbuild_builder.go:60`
  - build flow now discovers HTML assets and renders them after JS bundling at `internal/build/esbuild_builder.go:102`, `internal/build/esbuild_builder.go:120`, and `internal/build/esbuild_builder.go:157`
- Added HTML asset discovery, script `src` normalization, relative chrome-sim injection, and chrome-sim bundle emission:
  - `internal/build/html_assets.go:24`, `internal/build/html_assets.go:50`, `internal/build/html_assets.go:83`, `internal/build/html_assets.go:111`, `internal/build/html_assets.go:126`, and `internal/build/html_assets.go:180`
- Added repo-local auto-detection for `shared/chrome-sim/src/index.ts` and relevant `node_modules` roots:
  - `internal/build/chrome_sim.go:9`, `internal/build/chrome_sim.go:35`, and `internal/build/chrome_sim.go:49`
- Wired `panex dev` to enable chrome-sim injection for extension builds when the shared shim is available in the repo:
  - `cmd/panex/main.go:127`, `cmd/panex/main.go:143`, and `cmd/panex/main.go:153`
- Added Go tests for HTML output rewriting/injection and chrome-sim auto-detection:
  - `internal/build/esbuild_builder_test.go:103`, `internal/build/esbuild_builder_test.go:167`, and `internal/build/esbuild_builder_test.go:322`

## Risk and Mitigation
- Risk: nested HTML files could receive broken `chrome-sim.js` paths.
- Mitigation: injection computes relative module URLs per HTML output path and tests cover nested HTML pages.
- Risk: shared shim bundling could fail in isolated worktrees because dependencies live outside the extension source tree.
- Mitigation: auto-detection collects repo-local `node_modules` roots for both the extension package ancestry and `shared/chrome-sim`.

## Verification
- Commands run:
  - `GOCACHE=/tmp/go-build go test ./internal/build -count=1`
  - `GOCACHE=/tmp/go-build go test ./cmd/panex -count=1`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache golangci-lint run ./cmd/panex ./internal/build`
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
  - `GOCACHE=/tmp/go-build go test ./... -count=1`
- Expected:
  - builder tests pass for HTML copy, script rewrite, injection idempotency, and auto-detection.
  - `cmd/panex` tests still pass with the new builder option wiring.
  - full Go suite passes, including daemon integration tests.

## Teach-back (engineering lessons)
- Build-pipeline features should land in the shared builder once, not as per-surface scripts, when the behavior is meant to apply to all extension pages.
- HTML handling in JS bundlers is not free; if the builder owns `outDir`, it must also own HTML emission and path normalization.
- Auto-detection is acceptable for repo-local tooling when it is narrow, testable, and fails closed outside the expected layout.

## Next Step
- Extend the core build pipeline to copy remaining extension-facing static assets that HTML pages depend on, especially manifest-linked icons and other non-bundled files.

# PR39 - CI Trigger Coverage + Lint Unblock for `chrome.api.call` Routing

## Metadata
- Date: 2026-03-05
- PR: 39
- Branch: `feat/pr39-ci-checks-chrome-sim`
- Title: run CI on `feat/pr*` pushes, include `shared/chrome-sim` in TS checks, and fix `nilerr` lint failures
- Commit(s): pending

## Problem
- PR branches were not getting immediate CI on push, which made branch health harder to confirm before opening/refreshing PRs.
- The TypeScript CI matrix did not include `shared/chrome-sim`, so PR38 package checks would not appear in GitHub status even when workflows ran.
- Existing daemon `chrome.api.call` routing had intentional structured failure returns that tripped `golangci-lint` `nilerr`, causing `lint-and-test` to fail.

## Approach (with file+line citations)
- Expanded CI trigger coverage to run on feature PR branches as well as `main`:
  - added `feat/pr*` under `on.push.branches`
  - `.github/workflows/ci.yml:3-9`
- Extended TypeScript matrix to include `@panex/chrome-sim` package checks:
  - added matrix entry `{ name: chrome-sim, dir: shared/chrome-sim }`
  - `.github/workflows/ci.yml:51-60`
- Added lockfile path for cache dependency tracking:
  - `shared/chrome-sim/pnpm-lock.yaml`
  - `.github/workflows/ci.yml:75-79`
- Refactored storage-operation error mapping in daemon `chrome.api.call` handler to avoid `if err != nil { ... }, nil` patterns while preserving result semantics:
  - routed op errors through `chromeAPIFailureResult` helper
  - `internal/daemon/websocket_server.go:544-615`

## Risk and Mitigation
- Risk: enabling `feat/pr*` push CI increases workflow volume.
- Mitigation: scope is limited to the project’s PR branch naming convention rather than all branches (`.github/workflows/ci.yml:5-7`).
- Risk: matrix expansion can lengthen wall-clock CI time.
- Mitigation: TypeScript matrix already uses `fail-fast: false`, preserving per-package isolation and diagnostics (`.github/workflows/ci.yml:49-50`).
- Risk: lint-only refactor could accidentally alter protocol behavior for simulator call failures.
- Mitigation: helper preserves existing `success=false` + error-message result contract and only removes direct nilerr pattern (`internal/daemon/websocket_server.go:605-615`).

## Verification
- Commands run:
  - `git show -- .github/workflows/ci.yml`
  - `git diff -- .github/workflows/ci.yml`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache golangci-lint run ./internal/daemon`
- Expected:
  - workflow now triggers on `push` to `main` and `feat/pr*`.
  - TypeScript matrix includes `shared/chrome-sim`.
  - cache dependency paths include `shared/chrome-sim/pnpm-lock.yaml`.
  - daemon package lint no longer reports `nilerr` in `handleChromeAPICall`.

## Teach-back (engineering lessons)
- A package is only covered by CI if it is explicitly listed in the matrix; adding code without matrix updates silently bypasses checks.
- Feature-branch push triggers give faster feedback loops for maintainers who validate before PR status refresh.
- Keeping trigger scope narrow avoids unbounded CI cost while still improving reliability.

## Next Step
- Continue with PR40 to integrate `@panex/chrome-sim` into preview bootstrap/injection and start wiring additional simulated namespaces.

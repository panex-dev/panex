# PR Build Log

## Metadata
- Date: 2026-03-06
- PR: 49
- Branch: `feat/pr49-root-build-ci`
- Title: align root commands and CI with the polyglot build surface
- Commit(s):
  - `build(ci): validate root Go and TypeScript build flows`

## Problem
- The top-level README described `make test` and `make build` as repo-wide workflows, but the Makefile only exercised Go.
- CI type-checked and tested TypeScript packages, but it did not validate the daemon build or the frontend bundle outputs that can break independently.

## Approach (with file+line citations)
- Change 1:
  - Why: add root-level TypeScript `check`, `test`, and `build` orchestration so the top-level Makefile reflects the actual repo surface.
  - Where: `Makefile`
- Change 2:
  - Why: document the new root workflow and the one-time TypeScript dependency bootstrap needed before using it.
  - Where: `README.md`
- Change 3:
  - Why: make CI build the Go daemon and the TypeScript packages that have real build steps (`agent`, `inspector`).
  - Where: `.github/workflows/ci.yml`

## Risk and Mitigation
- Risk: root `make build` and `make test` now require TypeScript package dependencies to be installed locally, which raises the bar for a fresh checkout.
- Mitigation: the README now documents the required `pnpm --dir ... install` bootstrap, and CI already performs those installs explicitly per package.

## Verification
- Commands run:
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Additional checks:
  - Existing package-level `pnpm` scripts and Go integration tests passed through the top-level targets.

## Teach-back (engineering lessons)
- Design lesson: top-level developer commands must describe the real repo, not just the oldest part of it.
- Testing lesson: CI should validate build-producing surfaces separately from type-check/test surfaces, especially in polyglot repos.
- Team workflow lesson: documentation drift is often a tooling bug; if the root commands are incomplete, developers will keep following the wrong path.

## Next Step
- Fix JavaScript dependency determinism by eliminating the remaining no-lockfile install path and deciding whether these packages should live under a shared workspace.

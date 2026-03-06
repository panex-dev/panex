# PR51 - Root `pnpm` Workspace for TypeScript Packages

## Metadata
- Date: 2026-03-06
- PR: 51
- Branch: `feat/pr51-ts-workspace`
- Title: consolidate TypeScript installs under one root pnpm workspace lockfile
- Commit(s): pending

## Problem
- The repo managed four TypeScript packages as isolated `pnpm` projects with separate lockfiles and repeated install steps.
- That duplicated dependency-management work across local setup and CI, even though the package versions were already intentionally aligned.

## Approach (with file+line citations)
- Change 1:
  - Why: define a root `pnpm` workspace and pin the package manager version so all TypeScript dependency resolution happens through one audited entrypoint.
  - Where: `package.json:1-5`
  - Where: `pnpm-workspace.yaml:1-5`
  - Where: `pnpm-lock.yaml:1`
- Change 2:
  - Why: remove package-local lockfiles so determinism comes from the shared workspace lockfile instead of four separate files that can drift independently.
  - Where: `agent/pnpm-lock.yaml`
  - Where: `inspector/pnpm-lock.yaml`
  - Where: `shared/chrome-sim/pnpm-lock.yaml`
  - Where: `shared/protocol/pnpm-lock.yaml`
- Change 3:
  - Why: switch contributor guidance and CI caching/install behavior to the root workspace install path so local usage and automation follow the same dependency contract.
  - Where: `README.md:61-65`
  - Where: `.github/workflows/ci.yml:72-89`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:52-58`

## Risk and Mitigation
- Risk: workspace migration could leave one package accidentally relying on a package-local install layout.
- Mitigation: verification still runs package-level `check`, `test`, and `build` commands after a root `pnpm install --frozen-lockfile`, which exercises the real workspace layout.
- Risk: CI could keep caching against the obsolete per-package lockfiles and mask drift.
- Mitigation: the workflow now keys the `pnpm` cache from the root `pnpm-lock.yaml` and installs from the workspace root before running package commands.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm --dir shared/protocol run check`
  - `pnpm --dir shared/protocol run test`
  - `pnpm --dir agent run check`
  - `pnpm --dir agent run test`
  - `pnpm --dir agent run build`
  - `pnpm --dir inspector run check`
  - `pnpm --dir inspector run test`
  - `pnpm --dir inspector run build`
  - `pnpm --dir shared/chrome-sim run check`
  - `pnpm --dir shared/chrome-sim run test`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Expected:
  - Root workspace install succeeds with a frozen lockfile and creates the dependency layout used by all TS packages.
  - Every TypeScript package still passes its existing check/test/build surface without package-local lockfiles.
  - Root Go and TypeScript build flows continue to work unchanged after the workspace migration.

## Teach-back (engineering lessons)
- Design lesson: if package versions are already intentionally aligned, a shared workspace lockfile reduces operational duplication without changing package ownership boundaries.
- Testing lesson: install-path migrations need verification against the real post-install filesystem layout, not just lockfile diffs.
- Team workflow lesson: CI and contributor docs must move in the same PR as dependency-topology changes or the repo ends up with split-brain setup instructions.

## Next Step
- Decide whether repeated TypeScript compiler and script conventions should move into shared workspace-level config now that installs are unified.

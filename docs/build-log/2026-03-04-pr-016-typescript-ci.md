# PR16 - TypeScript CI Across Shared Protocol, Agent, and Inspector

## Metadata
- Date: 2026-03-04
- PR: 16
- Branch: `ci/typescript-lane`
- Title: add TypeScript CI job and missing shared protocol scripts
- Commit(s): pending

## Problem
- CI only enforced Go lint/test, so TypeScript breakage in the agent, inspector, or shared protocol could merge undetected.
- `shared/protocol/package.json` lacked `check`/`test` scripts, preventing consistent CI execution across all TS packages.

## Approach (with file+line citations)
- Added a matrix-based `typescript` CI job in GitHub Actions to run install + type-check + tests for:
  - `shared/protocol`, `agent`, and `inspector`.
  - Where: `.github/workflows/ci.yml:33-79`
- Added missing scripts for the shared protocol package:
  - `check`: `tsc --noEmit`
  - `test`: `node --import tsx --test tests/*.test.ts`
  - Where: `shared/protocol/package.json:7-10`
- Added explicit shared protocol dev toolchain dependencies required by those scripts:
  - `@types/node`, `tsx`, `typescript`
  - Where: `shared/protocol/package.json:15-18`
- Made CI install logic resilient to mixed package state (lockfile present vs absent):
  - `--frozen-lockfile` when `pnpm-lock.yaml` exists
  - `--no-frozen-lockfile` when it does not
  - Where: `.github/workflows/ci.yml:64-73`

## Risk and Mitigation
- Risk: package install behavior differs between packages with and without lockfiles.
- Mitigation: CI install path is explicit and deterministic per package state (`ci.yml:64-73`), and lockfile-backed packages remain frozen.
- Risk: local verification for `shared/protocol` may fail in restricted network environments.
- Mitigation: validated agent/inspector lanes locally and documented the shared/protocol DNS limitation; CI in GitHub-hosted runners is the primary gate for this lane.

## Verification
- Commands run:
  - `pnpm --dir agent run check && pnpm --dir agent run test`
  - `pnpm --dir inspector run check && pnpm --dir inspector run test`
  - `pnpm --dir shared/protocol install --offline --no-frozen-lockfile`
- Expected:
  - Agent and inspector checks/tests pass (observed pass).
  - Shared protocol install fails in this environment due missing offline tarball and DNS restrictions to npm registry.

## Teach-back (engineering lessons)
- Cross-language contract work (Go + TS) must be backed by cross-language CI, not only by local discipline.
- Monorepo-adjacent layouts without a single workspace lockfile need explicit CI install policies per package.
- It is better to add partial local verification plus clear constraints than to skip implementation waiting on perfect environment access.

## Next Step
- Run the updated CI workflow on GitHub to validate the `shared/protocol` lane in a network-enabled runner, then move to the next foundation increment.

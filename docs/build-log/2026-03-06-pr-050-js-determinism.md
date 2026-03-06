# PR Build Log

## Metadata
- Date: 2026-03-06
- PR: 50
- Branch: `feat/pr50-js-determinism`
- Title: require frozen lockfiles for every TypeScript package install
- Commit(s):
  - `build(ci): require frozen lockfiles for TS installs`

## Problem
- `shared/protocol` had no tracked `pnpm-lock.yaml`, so CI fell back to `pnpm install --no-frozen-lockfile`.
- That violated the repo’s deterministic-build goal and made one package materially less reproducible than the others.

## Approach (with file+line citations)
- Change 1:
  - Why: add the missing `shared/protocol/pnpm-lock.yaml` so the shared protocol package has the same locked dependency contract as the other TypeScript packages.
  - Where: `shared/protocol/pnpm-lock.yaml`
- Change 2:
  - Why: remove the CI fallback that allowed non-frozen installs and switch cache inputs to the new lockfile.
  - Where: `.github/workflows/ci.yml`

## Risk and Mitigation
- Risk: future dependency changes now require an intentional lockfile update instead of silently resolving in CI.
- Mitigation: that friction is deliberate; it converts hidden dependency drift into an explicit code review artifact.

## Verification
- Commands run:
  - `pnpm --dir shared/protocol install --frozen-lockfile`
  - `pnpm --dir agent install --frozen-lockfile`
  - `pnpm --dir inspector install --frozen-lockfile`
  - `pnpm --dir shared/chrome-sim install --frozen-lockfile`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build go test -race -count=1 ./...`
  - `GOCACHE=/tmp/go-build go build ./cmd/panex/...`
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

## Teach-back (engineering lessons)
- Design lesson: determinism is only real if the last unlocked path is removed; one exception is enough to reintroduce drift.
- Testing lesson: install-path changes should be validated by actually running installs, not inferred from existing node_modules.
- Team workflow lesson: lockfiles are part of the source of truth, not generated noise to be left out of review.

## Next Step
- Decide whether the four TypeScript packages should remain independently managed or move into a shared workspace to reduce duplicated dependency management.

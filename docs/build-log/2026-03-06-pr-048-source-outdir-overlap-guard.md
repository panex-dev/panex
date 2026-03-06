# PR Build Log

## Metadata
- Date: 2026-03-06
- PR: 48
- Branch: `feat/pr48-source-outdir-guard`
- Title: reject overlapping source and output directories
- Commit(s):
  - `fix(config): reject overlapping source and output paths`

## Problem
- The configuration layer only rejected `source_dir == out_dir`, not parent/child overlap such as `source_dir="./extension"` with `out_dir="./extension/dist"`.
- That left a common path layout able to feed build outputs back into the watcher and create self-triggered rebuild loops.

## Approach (with file+line citations)
- Change 1:
  - Why: reject overlapping extension paths in config validation before the daemon starts.
  - Where: `internal/config/config.go`
- Change 2:
  - Why: keep the builder defensive even if callers bypass config loading.
  - Where: `internal/build/esbuild_builder.go`
- Change 3:
  - Why: cover equality plus both overlap directions in config and builder tests.
  - Where: `internal/config/config_test.go`
  - Where: `internal/build/esbuild_builder_test.go`

## Risk and Mitigation
- Risk: configs that previously used nested output directories now fail fast.
- Mitigation: failure is intentional and happens before the daemon starts, with a specific validation error instead of a runtime rebuild loop.

## Verification
- Commands run:
  - `gofmt -w internal/config/config.go internal/config/config_test.go internal/build/esbuild_builder.go internal/build/esbuild_builder_test.go`
  - `GOCACHE=/tmp/go-build go test ./internal/config ./internal/build -count=1`
  - `GOCACHE=/tmp/go-build go test -race -count=1 ./internal/config ./internal/build`
  - `GOCACHE=/tmp/go-build go test -race -count=1 ./...`
  - `GOCACHE=/tmp/go-build go build ./cmd/panex`
- Additional checks:
  - `pnpm --dir shared/protocol run check`
  - `pnpm --dir shared/protocol run test`
  - `pnpm --dir agent run check`
  - `pnpm --dir agent run test`
  - `pnpm --dir inspector run check`
  - `pnpm --dir inspector run test`
  - `pnpm --dir shared/chrome-sim run check`
  - `pnpm --dir shared/chrome-sim run test`

## Teach-back (engineering lessons)
- Design lesson: path validation should reject the whole unsafe region, not just one exact invalid value.
- Testing lesson: overlap bugs need both directions covered because `source contains out` and `out contains source` fail for different reasons.
- Team workflow lesson: keeping the builder defensive as well as the config layer prevents future callers from reintroducing the same class of bug.

## Next Step
- Add repo-level build and CI coverage for the full polyglot toolchain so the documented root workflow matches what the project actually needs to stay green.

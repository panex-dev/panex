# PR78 - PANEX_AUTH_TOKEN Override

## Metadata
- Date: 2026-03-09
- PR: 78
- Branch: `feat/pr78-panex-auth-token-override`
- Title: support `$PANEX_AUTH_TOKEN` overrides in `panex dev`
- Commit(s):
  - `feat(cli): support PANEX_AUTH_TOKEN override`

## Problem
- `panex dev` required `server.auth_token` to come only from `panex.toml`, so local automation and packaging flows had to rewrite config files just to change daemon auth.
- The repo now documents the config contract, but there was still no supported runtime override for the auth token.

## Approach (with file+line citations)
- Change 1:
  - Why: keep config loading deterministic and file-only, but let the CLI apply a runtime auth override after config load so lower-level packages do not silently depend on process environment.
  - Where: `cmd/panex/main.go:114-165`
  - Where: `cmd/panex/main_test.go:181-251`
  - Where: `cmd/panex/main_test.go:686-693`
- Change 2:
  - Why: document the new override where contributors configure `panex.toml`, and move the roadmap forward once the slice is real.
  - Where: `README.md:22-63`
  - Where: `docs/build-log/STATUS.md:74-88`
  - Where: `docs/build-log/README.md:44-48`

## Risk and Mitigation
- Risk: an environment override could silently mask a misconfigured or empty auth token.
- Mitigation: `PANEX_AUTH_TOKEN` only overrides when present, and an explicitly empty/whitespace value fails fast with a CLI error instead of falling back silently.
- Risk: moving environment handling into the config loader would make lower-level behavior harder to reason about and test.
- Mitigation: the override lives in `panex dev` after `internal/config.Load(...)`, so the config package remains deterministic and unit-testable without ambient environment state.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - `GOCACHE=/tmp/go-build go test ./cmd/panex -run 'TestRunDevEnvAuthTokenOverride|TestRunDevRejectsEmptyEnvAuthTokenOverride' -count=1`

## Teach-back
- Design lesson: when runtime overrides are CLI concerns, keep them at the CLI boundary instead of teaching low-level config loaders about ambient environment.
- Safety lesson: override env vars should fail loudly on empty values, because silent fallback makes automation mistakes harder to detect.
- Workflow lesson: roadmap slices stay easier to execute when the prior docs PR already established the exact operator-facing surface being extended.

## Next Step
- Package reproducible release archives for the `panex` CLI as the first distribution-oriented slice.

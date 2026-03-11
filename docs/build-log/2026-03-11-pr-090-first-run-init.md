# PR90 Build Log: First-run init scaffolding

## Metadata
- Date: 2026-03-11
- PR: 90
- Branch: `feat/pr90-first-run-init`
- Title: `feat(cli): add first-run init scaffolding`
- Commit(s): `feat(cli): add first-run init scaffolding (#90)`

## Problem
- Panex still made first-time use too manual: operators had to hand-write `panex.toml`, pick a source tree, and create their own starter extension before `panex dev` could even start.
- The README quick start pointed at `examples/hello-extension`, which did not exist in the repository, so the documented path to “first success” was not real.

## Approach (with file+line citations)
- Change 1:
  - Why: add a dedicated `panex init` command that scaffolds a working `panex.toml` plus a visible starter extension in the current directory, with a `--force` path for regenerating those starter files.
  - Where: `cmd/panex/main.go:22-27`
  - Where: `cmd/panex/main.go:98-165`
  - Where: `cmd/panex/init.go:13-175`
- Change 2:
  - Why: turn missing-config detection into a stable sentinel error so `panex dev` can give default-path users a direct `panex init` recovery path without guessing from free-form strings.
  - Where: `internal/config/config.go:14-71`
  - Where: `internal/config/config_test.go:326-337`
- Change 3:
  - Why: lock the new first-run path with command-level tests for scaffolding, overwrite protection, `init -> dev`, and missing-default-config guidance, then rewrite the README quick start around the new flow.
  - Where: `cmd/panex/main_test.go:62-200`
  - Where: `cmd/panex/main_test.go:393-412`
  - Where: `README.md:18-176`

## Risk and Mitigation
- Risk: scaffold generation could overwrite user work if it is too aggressive.
- Mitigation: `panex init` refuses to replace `panex.toml` or `panex-extension` unless the operator explicitly passes `--force`.
- Risk: this PR could imply Panex now automates Chrome-side extension installation.
- Mitigation: the command output and README still state the remaining manual step clearly: load `.panex/dist` with Chrome’s `Load unpacked`.

## Verification
- Commands run:
  - `GOCACHE=/tmp/go-build go test ./cmd/panex ./internal/config -count=1`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - The new command-level test suite covers `panex init`, `panex init --force`, and `panex init` followed by `panex dev`.

## Teach-back (engineering lessons)
- Design lesson: the fastest way to cut first-run friction is not broad auto-detection, it is one opinionated happy path that creates a real working project shape.
- Testing lesson: onboarding features need flow tests, not just file-write unit tests; `init -> dev` is the product contract here.
- Team workflow lesson: if the README points at a first-run path that does not exist in the repo, that is a product bug, not just a docs bug.

## Next Step
- Decide whether the next release-readiness slice should reduce the remaining manual browser step, or whether the current first-run experience is now good enough to cut the next release candidate.

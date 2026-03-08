# PR62 - Make Init Bootstrap

## Metadata
- Date: 2026-03-09
- PR: 62
- Branch: `fix/pr62-make-init-workflow`
- Title: add a supported `make init` bootstrap for git hook installation
- Commit(s): pending

## Problem
- The audit still had an open process gap: the repo shipped `scripts/install-git-hooks.sh`, but there was no stable bootstrap command for contributors to discover and run.
- That left branch-base guard enforcement dependent on tribal knowledge instead of a documented entrypoint.

## Approach (with file+line citations)
- Change 1:
  - Why: add a first-class `make init` target so hook installation lives in the repo's standard command surface instead of a side script.
  - Where: `Makefile:1-14`
- Change 2:
  - Why: move setup and hook-install documentation onto `make init` so the supported bootstrap path is obvious in the repo root docs.
  - Where: `README.md:11-12`
  - Where: `README.md:38-41`
- Change 3:
  - Why: mark the audit item closed and keep the remaining unresolved items explicit.
  - Where: `audit.md:5-25`
  - Where: `docs/build-log/STATUS.md:59-63`
  - Where: `docs/build-log/2026-03-09-pr-062-make-init-bootstrap.md:1-48`

## Risk and Mitigation
- Risk: `make init` could imply broader environment bootstrapping than it actually performs.
- Mitigation: the target stays intentionally narrow and only installs git hooks; the README still documents TypeScript dependency installation separately.
- Risk: verification could mutate the shared repo config unexpectedly.
- Mitigation: the verification checks the command directly and then inspects the effective `core.hooksPath`, which is exactly the repo-level state this target is meant to establish.

## Verification
- Commands run:
  - `make init`
  - `git config --get core.hooksPath`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Expected:
  - `git config --get core.hooksPath` prints `.githooks`

## Teach-back
- Design lesson: a process guard is only real if the repo exposes one obvious command to enable it.
- Testing lesson: for workflow/bootstrap changes, verifying the persisted repo state is more valuable than trying to unit-test shell glue indirectly.
- Workflow lesson: keep bootstrap commands narrow; conflating hooks, dependency installation, and local environment setup under one target makes failures harder to reason about.

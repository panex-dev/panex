# PR95 Build Log: Add `panex paths` for quick path inspection

## Metadata
- Date: 2026-03-16
- PR: 95
- Branch: `feat/pr95-panex-paths`
- Title: `feat(cli): add panex paths for quick path inspection`
- Commit(s): `feat(cli): add panex paths for quick path inspection (#95)`

## Problem
- Users had no quick way to get absolute source and output directory paths without starting a dev server or running the full `panex doctor` diagnostic.
- Scripts and CI pipelines need machine-parseable path output (e.g. `panex paths | grep out_dir | cut -d= -f2`).

## Approach (with file+line citations)
- Change 1:
  - Why: add a `panex paths` command that loads or infers config and prints absolute source and output paths in a key=value format.
  - Where: `cmd/panex/paths.go:9-46` (`runPaths` function)
  - Where: `cmd/panex/main.go:30` (usage text)
  - Where: `cmd/panex/main.go:119` (case "paths" in command switch)
- Change 2:
  - Why: reuse `detectConfig` from `doctor.go` (same package) to avoid duplicating config loading logic.
  - Where: `cmd/panex/paths.go:10` (calls `detectConfig()`)

## Risk and Mitigation
- Risk: `filepath.Abs` could fail if the working directory has been deleted.
- Mitigation: on error, the raw config path is used as fallback.

## Verification
- Commands run:
  - `make fmt`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Additional checks:
  - Tests cover: panex.toml, manifest.json inference, multi-extension bracket labels, no-config error, and command routing via `run(["paths"])`.

## Teach-back (engineering lessons)
- Design lesson: utility commands that print machine-parseable output (`key=value`) serve both human readers and automation consumers with the same interface. No need for `--json` or `--format` flags at this scale.

## Next Step
- Start Phase 3: `panex dev --open` for Chrome boundary assistance, or begin Phase 4 packaging work.

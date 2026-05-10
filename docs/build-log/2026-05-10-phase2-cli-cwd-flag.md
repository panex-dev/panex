# Phase 2 CLI surface — add global `--cwd` project resolution override

**Status:** PR pending
**Date:** 2026-05-10

## Problem

The Phase 1 CLI still resolved project state from the process working directory only. `panex dev`, `inspect`, `plan`, `apply`, `test`, `verify`, `package`, `report`, `resume`, `doctor`, `paths`, `mcp`, and `init` all assumed the caller had already `cd`'d into the project root, which left the spec's global `--cwd` override unimplemented.

That gap was larger than simple command dispatch. Even if the CLI found `panex.toml` under an alternate directory, relative `source_dir`, `out_dir`, and `event_store_path` values would still resolve against the shell's current directory instead of the config directory.

## Approach

- Added top-level global flag parsing in `cmd/panex/main.go` so `--cwd` is resolved before subcommand dispatch.
- Routed the resolved project directory through every cwd-sensitive command entrypoint in `cmd/panex/main.go`, `cmd/panex/core.go`, `cmd/panex/init.go`, `cmd/panex/doctor.go`, and `cmd/panex/paths.go`.
- Normalized `panex.toml`-relative paths against the config directory before `panex dev` starts the runtime, so `source_dir`, `out_dir`, and `event_store_path` remain correct when the CLI is invoked from outside the project root.
- Added regression coverage in `cmd/panex/main_test.go` for `--cwd` on `init`, `dev`, and `paths`, plus invalid global-flag and invalid-`--cwd` handling.

## Risk and mitigation

- Risk: changing dev config loading could alter existing relative-path behavior for users already running from the project root.
- Mitigation: the runtime still points at the same on-disk locations; the only change is that loaded config paths are normalized to absolute project-rooted paths before downstream build and doctor logic uses them. Existing in-root flows stay covered by the `cmd/panex` test package and the full repo gates.

- Risk: global flag parsing could break help or subcommand flag parsing.
- Mitigation: the parser stops at the first non-flag command token, keeps `help`/`-h`/`--help` behavior intact, and the tests cover invalid global flags separately from subcommand-local flag errors.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `GOCACHE=/tmp/go-build go test ./cmd/panex/...`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the remaining Phase 2 spec-gap inventory with another narrow CLI or MCP surface slice, rather than broadening this PR into unrelated deferred features.

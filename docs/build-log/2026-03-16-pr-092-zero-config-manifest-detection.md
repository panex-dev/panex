# PR92 Build Log: Zero-config `panex dev` for manifest.json directories

## Metadata
- Date: 2026-03-16
- PR: 92
- Branch: `feat/pr92-zero-config-manifest-detection`
- Title: `feat(config): zero-config panex dev for manifest.json directories`
- Commit(s): `feat(config): zero-config panex dev for manifest.json directories (#92)`

## Problem
- Running `panex dev` in a directory containing a Chrome extension (`manifest.json`) required creating a `panex.toml` config file first, even though sensible defaults exist.
- The source/output overlap validation rejected `source_dir="."` + `out_dir=".panex/dist"` as an overlap, even though PR91 ensured all source walkers and file watchers skip infrastructure directories like `.panex/`. This forced users to separate source and output into sibling directories.

## Approach (with file+line citations)
- Change 1:
  - Why: relax the source/output overlap validation so that output directories nested inside source are allowed when the nesting path begins with an infrastructure directory (dot-prefixed or `node_modules`). These directories are already skipped by all discovery functions and file watchers (PR91), so the overlap cannot cause build loops.
  - Where: `internal/config/config.go:271-298` (directional `pathsOverlap` with `isShieldedByInfrastructureDir`)
  - Where: `internal/config/config.go:311-330` (`isShieldedByInfrastructureDir` and `isInfrastructureDir` predicates)
  - Where: `internal/build/esbuild_builder.go:102-126` (matching directional `pathsOverlap`)
  - Where: `internal/build/esbuild_builder.go:139-147` (`isShieldedByInfrastructureDir` predicate)
- Change 2:
  - Why: add convention-based config inference so that a directory containing `manifest.json` can be used with `panex dev` without a config file. The inferred config uses the directory as `source_dir`, `.panex/dist` as `out_dir`, port 4317, and `dev-token` as auth token.
  - Where: `internal/config/config.go:14-15` (`ErrManifestNotFound` sentinel)
  - Where: `internal/config/config.go:22-24` (`DefaultPort`, `DefaultOutDir`, `DefaultAuthToken` constants)
  - Where: `internal/config/config.go:74-106` (`Infer` function)
- Change 3:
  - Why: wire the config inference into the CLI so that `panex dev` automatically falls back to `Infer(".")` when no `panex.toml` exists, before suggesting `panex init`.
  - Where: `cmd/panex/main.go:143-159` (inference fallback in `runDev`)

## Risk and Mitigation
- Risk: a user with `source_dir="."` and `out_dir=".custom-output"` might not realize the output is shielded by the dot-prefix convention. If they expect overlap rejection, they won't get it.
- Mitigation: dot-prefixed output directories are explicitly infrastructure by convention. The shielding only applies when the *first* path component is an infrastructure directory. Regular nesting like `out_dir="./dist"` is still rejected.
- Risk: the inferred `dev-token` auth token is predictable.
- Mitigation: this is a local development tool. The daemon only binds to loopback (127.0.0.1). Users can override via `$PANEX_AUTH_TOKEN` or by creating a `panex.toml`.

## Verification
- Commands run:
  - `make fmt`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - New config tests verify `Infer` succeeds with manifest.json, fails with `ErrManifestNotFound` when absent, and rejects empty directory argument.
  - New overlap tests verify infrastructure-shielded output directories (`.panex/dist`) are accepted while regular nesting (`./extension/dist`) is still rejected.
  - New CLI tests verify inferred config is used when manifest.json exists without panex.toml, and that `$PANEX_AUTH_TOKEN` overrides apply to inferred configs.
  - Existing overlap rejection tests all still pass unchanged.

## Teach-back (engineering lessons)
- Design lesson: relaxing a safety check requires proving the invariant still holds. The overlap guard exists to prevent build loops; PR91's infrastructure directory exclusion makes the invariant hold for dot-prefixed nesting, so the relaxation is safe.
- API lesson: `Infer` calls `Validate` on the constructed config. This is defense-in-depth — if the inference logic ever drifts from the validation rules, it fails fast rather than producing a silently broken config.
- UX lesson: printing a notice ("no panex.toml found, using manifest.json in current directory") when inference activates prevents user confusion about where config values came from.

## Next Step
- Wire zero-config `panex dev` end-to-end for the demo experience: `cd my-extension && panex dev` should build, watch, and serve without any prior setup.

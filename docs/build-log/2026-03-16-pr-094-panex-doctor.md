# PR94 Build Log: Add `panex doctor` for project health checks

## Metadata
- Date: 2026-03-16
- PR: 94
- Branch: `feat/pr94-panex-doctor`
- Title: `feat(cli): add panex doctor for project health checks`
- Commit(s): `feat(cli): add panex doctor for project health checks (#94)`

## Problem
- Users had no way to diagnose common setup issues without reading documentation. If `panex dev` failed or produced unexpected results, the only recovery path was manual investigation.
- WSL users had no warning when their output path was invisible to Windows Chrome, leading to confusing "Load unpacked" failures.
- The onboarding plan (Phase 2) calls for `panex doctor` as a guided troubleshooting command.

## Approach (with file+line citations)
- Change 1:
  - Why: add a `panex doctor` command that checks project health: config detection, source/output directory existence, build completeness (manifest.json in output), and WSL path visibility.
  - Where: `cmd/panex/doctor.go:15-98` (`runDoctor` function with all checks)
  - Where: `cmd/panex/doctor.go:100-115` (`detectConfig` tries Load then Infer)
  - Where: `cmd/panex/doctor.go:121-129` (`isWSL` reads /proc/version for Microsoft/WSL markers)
- Change 2:
  - Why: wire the command into the CLI dispatcher and update usage text.
  - Where: `cmd/panex/main.go:28` (usage text)
  - Where: `cmd/panex/main.go:115` (case "doctor" in command switch)

## Risk and Mitigation
- Risk: `/proc/version` may not exist on non-Linux systems.
- Mitigation: `defaultReadProcVersion` returns nil on read error, causing `isWSL()` to return false. The check is a no-op on macOS and Windows.
- Risk: `filepath.Abs` could fail if the working directory is deleted.
- Mitigation: on error, the raw config path is used instead. This is a diagnostic tool — degraded output is better than a crash.

## Verification
- Commands run:
  - `make fmt`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Additional checks:
  - Tests cover: panex.toml present, manifest.json inferred, no config, output exists without manifest, WSL warning, no WSL warning for non-WSL, and command routing via `run(["doctor"])`.
  - WSL detection is mockable via `readProcVersion` function variable, matching the existing stub pattern for `lookupEnv` and `newSignalContext`.

## Teach-back (engineering lessons)
- UX lesson: diagnostic commands should always exit successfully (exit 0) and report issues in human-readable text. A failing diagnostic tool is worse than no diagnostic tool.
- Testing lesson: platform-specific checks (like WSL detection) need injectable dependencies from the start. Adding a function variable for `/proc/version` reads is cheap and makes the tests deterministic.

## Next Step
- Add `panex paths` command for quick source/output directory inspection, or start Phase 3 (`panex dev --open`) for Chrome boundary assistance.

# PR93 Build Log: Guided startup messaging with Chrome loading instructions

## Metadata
- Date: 2026-03-16
- PR: 93
- Branch: `feat/pr93-guided-startup-messaging`
- Title: `feat(cli): guided startup messaging with Chrome loading instructions`
- Commit(s): `feat(cli): guided startup messaging with Chrome loading instructions (#93)`

## Problem
- `panex dev` printed only `panex dev` and `ws_url=...` at startup. New users had no guidance on what to do next — they needed to know the output directory path and the exact Chrome steps to load their extension.
- The onboarding plan requires every first-run path to report: project path, output path, and exact next user action.

## Approach (with file+line citations)
- Change 1:
  - Why: print the resolved output directory path and Chrome loading instructions in the startup banner so users know exactly where the build output goes and how to load it in Chrome.
  - Where: `cmd/panex/main.go:216` (call `writeStartupGuide` after existing banner)
  - Where: `cmd/panex/main.go:199-220` (`writeStartupGuide` function)
- Change 2:
  - Why: for multi-extension configs, print each extension's output path with its ID label. Chrome loading instructions are omitted for multi-extension mode since those users are power users.
  - Where: `cmd/panex/main.go:206-210` (multi-extension `out_dir[id]=path` format)

## Risk and Mitigation
- Risk: the output path is resolved via `filepath.Abs` at startup, before the build creates the directory. If the cwd changes between startup and Chrome loading, the path might be stale.
- Mitigation: `filepath.Abs` resolves at the moment of printing, which matches the build's resolution. The cwd does not change during `panex dev` execution.

## Verification
- Commands run:
  - `make fmt`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Additional checks:
  - Updated `TestStartDevServerCoordinatesStartupLifecycle` to verify `out_dir=` line and Chrome loading guide are present in startup output.
  - Multi-extension test (`TestStartDevServerStartsOneBuilderAndWatcherPerExtension`) uses `io.Discard` and is unaffected.

## Teach-back (engineering lessons)
- UX lesson: startup banners should answer "what do I do next?" for beginners while remaining parseable (key=value lines) for automation. Mixing both formats in one output serves both audiences.
- Testing lesson: banner tests should use `strings.Contains` for structural assertions rather than exact string comparison, since absolute paths vary by environment.

## Next Step
- Add `panex doctor` and `panex paths` helper commands for guided troubleshooting and path inspection.

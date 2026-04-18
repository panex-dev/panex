# Phase 1 CI Green — cross-platform fixes

**Status:** PR pending (extends `2026-04-18-phase1-lint-cleanup.md`)
**Date:** 2026-04-18

## Problem

After the lint cleanup commit landed `lint` green, three other CI checks remained red on `main` itself, blocking a strict-green merge:

1. `dependency-verification` (govulncheck) — Go 1.25.8 stdlib has three vulnerabilities (`GO-2026-4870`, `GO-2026-4946`, `GO-2026-4947`) in `crypto/x509` and `crypto/tls`, fixed in 1.25.9.
2. `go-test (macos-latest)` — `TestPathsWithPanexToml` compares an absolute expected path computed from `t.TempDir()` against output produced after `os.Chdir(tempDir)`. macOS resolves `/var → /private/var`, so the post-chdir path differs from the pre-chdir expectation.
3. `go-test (windows-latest)` — three failure modes: hard-coded forward slashes in path assertions (`.panex/dist`); a WSL-specific test (`/mnt/c/...`) that has no meaningful semantics on native Windows; and `TestCmdInit_NewProject` timing out at 10 minutes because `chrome.exe --version` does not print to stdout and never exits.

## Approach

- **Go 1.25.8 → 1.25.9** in `.github/workflows/ci.yml`, `.github/workflows/release.yml`, and `go.mod`.
- **`cmd/panex/paths_test.go`** — added `resolveSymlinks(t, path)` helper; `TestPathsWithPanexToml` now calls it on `t.TempDir()` so the expected and actual paths share the same realpath. Replaced literal `".panex/dist"` with `filepath.Join(".panex", "dist")` in `TestPathsWithManifestJSON`.
- **`cmd/panex/main_test.go`** — `TestRunDevInfersConfigFromManifestJSON` compares against `filepath.FromSlash(panexconfig.DefaultOutDir)` so Windows backslashes match. `TestRunDevNoWSLWarningWhenOutDirUnderMnt` skips on Windows (the `/mnt/` path becomes literal `C:\mnt\c\...` under native Windows, defeating the WSL detection logic the test exercises).
- **`internal/target/chrome.go:80-90`** — `InspectEnvironment` now wraps the `chrome --version` exec in a 3-second timeout context. On Windows, `chrome.exe --version` does not write to stdout (and on some builds opens a new window); without a deadline the call blocks indefinitely and times out the parent test (`TestCmdInit_NewProject` calls `detectEnvironment` → `InspectEnvironment`). The 3s bound is generous for a normal `--version` print and short enough not to stall CI.

## Risk and Mitigation

- **Risk (Go bump):** Go 1.25.9 may introduce minor behavior changes beyond the security fix.
- **Mitigation:** Patch release within the 1.25.x line; release notes only describe the listed CVE fixes, no behavior changes.
- **Risk (chrome `--version` timeout):** if Chrome legitimately takes longer than 3 s on a slow runner, `info.Version` will be empty.
- **Mitigation:** `info.Available` and `info.BinaryPath` are set unconditionally. `Version` is best-effort metadata and not load-bearing for any caller. The `Result.Outcome` remains `Success` either way.

## Verification

- `make fmt` — clean
- `make lint` — clean
- `go test -race -count=1 ./internal/... ./cmd/panex/...` — 24/24 pass on linux/amd64
- macOS and Windows verification deferred to CI (cannot reproduce locally).

## Next Step

Once green, merge the combined PR. Phase 2 planning resumes after.

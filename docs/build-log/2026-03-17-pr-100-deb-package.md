# PR100 Build Log: Add `.deb` package generation for Linux releases

## Metadata
- Date: 2026-03-17
- PR: 100
- Branch: `feat/pr100-deb-package`
- Title: `feat(release): generate .deb packages for Linux targets`

## Problem
- Linux users on Debian/Ubuntu had to manually extract tarballs and move binaries to PATH.
- The onboarding plan calls for `.deb` packages as a step toward `apt install panex`.

## Approach (with file+line citations)
- Change 1:
  - Why: generate `.deb` packages in pure Go with no external `dpkg-deb` dependency.
  - Where: `internal/release/deb.go:1-178`
  - The `.deb` format is an `ar(5)` archive with three members: `debian-binary` (version marker), `control.tar.gz` (package metadata), and `data.tar.gz` (file tree).
  - Installs to `/usr/local/bin/panex` with deterministic timestamps for reproducibility.
- Change 2:
  - Why: wire `.deb` generation into the release packager for Linux targets.
  - Where: `cmd/panex-release/main.go:70-83` (deb generation in release loop)
  - Where: `cmd/panex-release/main.go:172-210` (`packageDeb` and `writeDebFile` functions)
  - `.deb` files are included in the SHA256SUMS checksum manifest alongside tarballs.
- Change 3:
  - Why: test the `.deb` structure, determinism, and edge cases.
  - Where: `internal/release/deb_test.go:1-182` (6 tests: architecture filtering, naming, ar structure, control metadata, data content, determinism, non-Linux rejection)
  - Where: `cmd/panex-release/main_test.go:45-98` (updated to verify `.deb` determinism and checksum inclusion on Linux)

## Risk and Mitigation
- Risk: the pure Go `.deb` builder doesn't use `dpkg-deb`, so it could produce non-conformant packages.
- Mitigation: the test verifies the exact ar layout, control fields, and data tree that `dpkg` expects. The format is minimal but correct — `debian-binary`, `control.tar.gz`, `data.tar.gz` in that order with GNU tar format.

## Verification
- Commands run:
  - `make fmt && make lint` — clean
  - `go test -race ./internal/release/...` — 11 tests pass
  - `go test -race ./cmd/panex-release/...` — 2 tests pass (including determinism)
  - `make test && make build` — full suite passes
  - `go run ./cmd/panex-release --version v0.0.1-test --targets linux/amd64` — produces `.deb` + tarball + checksums

## Teach-back (engineering lessons)
- Design lesson: `.deb` files are simpler than they appear — an `ar` archive with exactly three members in a fixed order. Pure Go generation avoids a `dpkg-deb` build dependency and keeps the release pipeline cross-platform.

## Next Step
- Continue Phase 4: Windows MSI/winget packaging.

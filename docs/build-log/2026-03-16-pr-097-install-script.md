# PR97 Build Log: Add Linux/macOS install script

## Metadata
- Date: 2026-03-16
- PR: 97
- Branch: `feat/pr97-install-script`
- Title: `feat(install): add cross-platform install script for Linux and macOS`
- Commit(s): `feat(install): add cross-platform install script for Linux and macOS (#97)`

## Problem
- Users had to manually download release archives, extract them, move the binary to PATH, and verify checksums — a multi-step process that beginners frequently get wrong.
- The onboarding plan calls for a one-command install path: `curl -fsSL ... | sh`.

## Approach (with file+line citations)
- Change 1:
  - Why: provide a single install script that detects platform, downloads the right archive, verifies its checksum, and installs the binary.
  - Where: `scripts/install.sh:1-181` (full installer)
  - Key behaviors:
    - Detects OS (`uname -s` → linux/darwin) and arch (`uname -m` → amd64/arm64)
    - Resolves latest version via GitHub redirect, or accepts `--version` pin
    - Downloads archive + SHA256SUMS manifest from GitHub releases
    - Verifies checksum with `sha256sum` or `shasum` (macOS fallback)
    - Installs to `/usr/local/bin` (with sudo if needed) or `~/.local/bin` as fallback
    - Warns if install dir is not on PATH
    - Prints version, location, and next steps on success
- Change 2:
  - Why: cover argument parsing, platform detection, and error paths without requiring real GitHub releases.
  - Where: `scripts/install_test.sh:1-108` (8 tests)

## Risk and Mitigation
- Risk: the script runs as root when piped through `sudo sh`. Only `cp` and `chmod` execute with elevated privileges — no package management or system config changes.
- Risk: checksum verification depends on `sha256sum` or `shasum` being available. Both are standard on Linux and macOS respectively. The script fails explicitly if neither is found.
- Risk: GitHub API rate limits could block latest-version resolution. Mitigation: the redirect-based lookup (`/releases/latest`) does not count against the authenticated API rate limit. Users can also pin `--version` to skip the lookup entirely.

## Verification
- Commands run:
  - `sh scripts/install_test.sh` (8/8 pass)
  - `make fmt`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`

## Teach-back (engineering lessons)
- Design lesson: install scripts should verify checksums before extracting or executing anything. The two-file approach (archive + manifest) lets the script fail before any binary is placed on the filesystem.
- Design lesson: falling back from `/usr/local/bin` to `~/.local/bin` keeps the script usable without sudo while still warning about PATH — matching the behavior users expect from tools like `rustup`.

## Next Step
- PR98: Homebrew tap formula for `brew install panex-dev/tap/panex`.

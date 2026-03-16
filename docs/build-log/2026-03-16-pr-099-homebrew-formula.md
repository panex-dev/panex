# PR99 Build Log: Add Homebrew tap formula and automation

## Metadata
- Date: 2026-03-16
- PR: 99
- Branch: `feat/pr99-homebrew-formula`
- Title: `feat(install): add Homebrew tap formula and release automation`

## Problem
- macOS and Linux Homebrew users had no `brew install` path for Panex.
- The onboarding plan calls for `brew install panex-dev/tap/panex` as a primary macOS install channel.

## Approach (with file+line citations)
- Change 1:
  - Why: generate a platform-aware Homebrew formula from release checksums.
  - Where: `scripts/generate-homebrew-formula.sh:1-79`
  - Fetches SHA256SUMS from the GitHub release, extracts per-platform checksums, emits a Ruby formula with `on_macos`/`on_linux` blocks and CPU architecture branching.
- Change 2:
  - Why: automate pushing the formula to the `panex-dev/homebrew-tap` repo.
  - Where: `scripts/update-homebrew-tap.sh:1-59`
  - Clones the tap, writes the generated formula, commits and pushes. Idempotent — exits cleanly if formula is already up to date.
- Change 3:
  - Why: update the tap automatically on stable releases.
  - Where: `.github/workflows/release.yml:108-117`
  - Runs `update-homebrew-tap.sh` after publishing release assets. Skipped for prereleases (tags with `-`). Requires `HOMEBREW_TAP_TOKEN` secret for cross-repo push.

## Risk and Mitigation
- Risk: `HOMEBREW_TAP_TOKEN` secret may not be configured yet. Mitigation: the CI step emits a warning and exits 0 if the token is empty.
- Risk: formula generation depends on release assets being published first. Mitigation: the step runs after `gh release create`/`upload`.

## Verification
- Commands run:
  - `./scripts/generate-homebrew-formula.sh --version v0.1.0-rc.2` — produces valid Ruby formula
  - `./scripts/update-homebrew-tap.sh --version v0.1.0-rc.2` — pushed formula to `panex-dev/homebrew-tap`
  - `make fmt && make lint && make test && make build` all pass

## Teach-back (engineering lessons)
- Design lesson: Homebrew formulas for prebuilt binaries use `on_macos`/`on_linux` + `Hardware::CPU.arm?` to select the right archive URL per platform — no compilation step needed.
- Ops lesson: cross-repo pushes from CI require a separate PAT with repo scope. Using `${{ github.token }}` only grants access to the current repo.

## Next Step
- Configure `HOMEBREW_TAP_TOKEN` secret on the panex repo for automated tap updates.

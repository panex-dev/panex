# PR81 - Release SHA256 Checksums

## Metadata
- Date: 2026-03-10
- PR: 81
- Branch: `feat/pr81-release-checksums`
- Title: publish SHA256 checksums alongside tagged release assets
- Commit(s):
  - `build(release): emit SHA256 checksum manifest`

## Problem
- PR80 publishes tagged release archives, but it does not provide a checksum manifest for operators to verify downloads after release.
- That leaves release consumers without a first-class integrity artifact even though the release pipeline already has one deterministic output directory.

## Approach (with file+line citations)
- Change 1:
  - Why: extend the release package and packager command to compute per-archive SHA256 digests and write one sorted `panex_<version>_SHA256SUMS` manifest beside the generated archives.
  - Where: `internal/release/release.go:1-180`
  - Where: `cmd/panex-release/main.go:1-209`
- Change 2:
  - Why: lock the checksum behavior with deterministic unit coverage at both the release-helper and packager layers.
  - Where: `internal/release/release_test.go:1-191`
  - Where: `cmd/panex-release/main_test.go:1-98`
- Change 3:
  - Why: document that tagged releases now publish checksum manifests and move the roadmap to the next consumption-side release slice.
  - Where: `README.md:100-112`
  - Where: `docs/build-log/STATUS.md:80-87`
  - Where: `docs/build-log/README.md:44-46`

## Risk and Mitigation
- Risk: checksum lines could vary across runs if archive ordering depends on target input order.
- Mitigation: the checksum manifest sorts archive names before writing lines, so repeated runs produce identical bytes even when targets are passed in a different order.
- Risk: the packager could emit checksum data that does not match the actual published archive bytes.
- Mitigation: the command computes checksums from the written archive files themselves, and tests assert that the manifest line matches the archive bytes on disk.
- Risk: release publishing could need more workflow changes than necessary.
- Mitigation: this slice reuses PR80 unchanged; the existing workflow already uploads every file under `dist/release/*`, so adding the manifest at the packager boundary is enough.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `GOCACHE=/tmp/go-build go test ./internal/release ./cmd/panex-release -count=1`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `GOCACHE=/tmp/go-build VERSION=v0.0.1-test TARGETS=$(go env GOOS)/$(go env GOARCH) make release`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - `sha256sum dist/release/panex_v0.0.1-test_linux_amd64.tar.gz`
  - `cat dist/release/panex_v0.0.1-test_SHA256SUMS`

## Teach-back
- Design lesson: when a release workflow already publishes one deterministic output directory, the cleanest integrity feature is to add new artifacts at the packager boundary instead of branching workflow logic.
- Testing lesson: checksum coverage should assert against the real written archive bytes, not against recomputed in-memory assumptions, otherwise the manifest can drift from the published asset.
- Workflow lesson: release follow-ons stay reviewable when each PR adds one more artifact to the same `make release` contract rather than layering separate publishing codepaths.

## Next Step
- Document download verification against published release checksums.

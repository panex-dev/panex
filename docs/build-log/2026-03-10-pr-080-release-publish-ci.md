# PR80 - Tagged CI Release Publishing

## Metadata
- Date: 2026-03-10
- PR: 80
- Branch: `feat/pr80-release-publish-ci`
- Title: publish tagged CI release artifacts on top of the local release packager
- Commit(s):
  - `build(release): publish tagged CI artifacts`

## Problem
- PR79 added a deterministic local release packager, but tagged releases still had no supported CI path to verify the tree, run the packager, and publish the generated archives.
- That left release publication as a manual, error-prone step and risked future release automation drifting away from the packager contract we had just established.

## Approach (with file+line citations)
- Change 1:
  - Why: add a dedicated tag-triggered `Release` workflow that only publishes from tags on `origin/main`, reruns the real verification gates, calls `make release VERSION=<tag>`, uploads the generated archives as workflow artifacts, and creates or updates the matching GitHub release.
  - Where: `.github/workflows/release.yml:1-91`
- Change 2:
  - Why: document the tag-cutting flow and make the release boundary explicit to contributors using the new packager.
  - Where: `README.md:102-113`
- Change 3:
  - Why: move the roadmap forward from “publish tagged CI release artifacts” to the next release-integrity slice and record this increment in project memory.
  - Where: `docs/build-log/STATUS.md:80-87`
  - Where: `docs/build-log/README.md:44-46`

## Risk and Mitigation
- Risk: a tag pushed from an unrelated branch could publish unreviewed release assets.
- Mitigation: the workflow fetches `origin/main` and verifies the tagged commit is an ancestor before any packaging or publishing begins.
- Risk: release automation could silently bypass the repository’s normal quality gates.
- Mitigation: the workflow reruns dependency verification, TypeScript checks, Go linting, tests, and builds before calling `make release`.
- Risk: rerunning a partially failed release could leave asset publication in a broken state.
- Mitigation: the publish step is idempotent enough for reruns: it creates the release when missing and otherwise re-uploads assets with `--clobber`.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `GOCACHE=/tmp/go-build VERSION=v0.0.1-test TARGETS=$(go env GOOS)/$(go env GOARCH) make release`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Reviewed the `Release` workflow to confirm it reuses `make release` rather than duplicating archive logic in YAML.

## Teach-back
- Design lesson: once release packaging has one tested entrypoint, CI should orchestrate that entrypoint instead of reimplementing versioning and target rules in workflow steps.
- Testing lesson: release automation still needs real verification gates before publication; packaging a tagged commit without rerunning checks just moves risk downstream.
- Workflow lesson: tag-driven release jobs should assert their commit is on `main`, otherwise a single mistag can publish code that never passed the normal review path.

## Next Step
- Publish SHA256 checksums alongside tagged release assets.

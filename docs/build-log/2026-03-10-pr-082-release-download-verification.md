# PR82 - Release Download Verification Docs

## Metadata
- Date: 2026-03-10
- PR: 82
- Branch: `docs/pr82-release-download-verification`
- Title: document download verification against published release checksums
- Commit(s):
  - `docs(release): add checksum verification examples`

## Problem
- Tagged releases now publish checksum manifests, but the README does not yet show operators how to use them to verify downloaded assets.
- That leaves the integrity feature incomplete in practice because consumers still have to guess the verification commands for their platform.

## Approach (with file+line citations)
- Change 1:
  - Why: add concrete Linux, macOS, and PowerShell verification examples that use the published `panex_<version>_SHA256SUMS` file against one downloaded asset at a time.
  - Where: `README.md:113-139`
- Change 2:
  - Why: move the release-integrity roadmap forward now that the download-verification docs are in place and record the closeout in project memory.
  - Where: `docs/build-log/STATUS.md:81-88`
  - Where: `docs/build-log/README.md:44-46`

## Risk and Mitigation
- Risk: platform-specific examples could be wrong or too implicit for operators to adapt.
- Mitigation: the docs show one explicit example per platform family and call out that both the version and filename should be replaced with the actual downloaded asset.
- Risk: a docs-only PR could accidentally overcommit the roadmap to a larger next milestone.
- Mitigation: the tracker now says to select the next post-release milestone from the queued follow-ons instead of guessing a broader engineering scope in this documentation slice.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Reviewed the new examples against the checksum manifest format emitted by `panex-release` (`<sha256><space><space><filename>`).

## Teach-back
- Design lesson: a release-integrity feature is not finished when the artifact exists; it is finished when operators have one documented path to use it correctly.
- Testing lesson: docs-only changes can still justify the full quality gates when they advance the active roadmap and should remain merge-routine rather than “special case” work.
- Workflow lesson: when one milestone sequence ends, the tracker should acknowledge that explicitly instead of silently inventing a new large next step inside a narrow documentation PR.

## Next Step
- Select the next post-release milestone from the queued follow-ons.

# PR86 - Split README Audiences

## Metadata
- Date: 2026-03-10
- PR: 86
- Branch: `feat/pr86-docs-split-readme-audiences`
- Title: split product, contributor, and agent documentation entry points
- Commit(s):
  - `docs: split product and contributor entry points (#86)`

## Problem
- The root `README.md` mixed product usage, contributor workflow, release engineering, and coding-agent process in one long document.
- That made the first doc a user sees harder to trust and harder to scan, because operator guidance was buried beside internal branch/release instructions that only contributors or agents need.

## Approach (with file+line citations)
- Change 1:
  - Why: rewrite the root `README.md` so it explains what Panex is, how to run it, how the config works, and how to verify downloaded releases without carrying contributor-only workflow material.
  - Where: `README.md:1-124`
- Change 2:
  - Why: move contributor setup, verification expectations, branch workflow, and release workflow into a dedicated `CONTRIBUTING.md` aimed at humans working on the repo.
  - Where: `CONTRIBUTING.md:1-91`
- Change 3:
  - Why: add a separate repo map so contributors can find the right subsystem without overloading either the product README or the agent protocol.
  - Where: `docs/repo-map.md:1-42`
- Change 4:
  - Why: record the new docs split in the tracker and build log so the repo memory matches the new audience boundaries.
  - Where: `docs/build-log/STATUS.md:80-96`
  - Where: `docs/build-log/README.md:44-48`

## Risk and Mitigation
- Risk: moving workflow text out of `README.md` could hide important contributor steps if the new docs are not linked clearly.
- Mitigation: the rewritten `README.md` links directly to `CONTRIBUTING.md`, `docs/repo-map.md`, and `AGENTS.md`, and `CONTRIBUTING.md` points back to the product README and repo map.
- Risk: docs-only restructuring could accidentally drop release or config details users still need.
- Mitigation: the root README keeps the CLI surface, startup expectations, config reference, and release checksum verification while removing only contributor and agent process material.

## Verification
- Commands run:
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Read the rewritten `README.md`, `CONTRIBUTING.md`, and `docs/repo-map.md` together to confirm each audience now has one clear entry point without duplicated branch/agent protocol text.

## Teach-back
- Design lesson: a root README is more useful when it stays anchored on product intent and operator outcomes instead of trying to be the whole project handbook.
- Documentation lesson: audience splits work best when each document owns one job and cross-links explicitly to the neighboring documents that serve other readers.
- Workflow lesson: documentation structure changes still need build-log tracking, otherwise future contributors cannot tell whether a doc omission is accidental or intentional.

## Next Step
- Select the next post-release milestone from the remaining queued follow-ons.

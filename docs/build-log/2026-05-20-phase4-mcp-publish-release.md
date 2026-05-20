# Phase 4 MCP Publish Release

**Status:** implemented
**Date:** 2026-05-20

## What

Add the MCP `publish_release` tool and the first target adapter publish contract for policy-gated dry-run publishing over existing packaged artifacts.

## Why

The Phase 4 publish surface was still a deferred MCP gap. The spec requires publishing to remain separate from packaging, operate on existing artifacts, require policy approval and profile references, and emit structured publish evidence instead of performing hidden side effects.

## Approach

- Extended the target adapter contract with `PublishArtifact` plus typed publish options and records.
- Implemented Chrome publish behavior as explicit dry-run success or clear blocked outcomes for missing profile, missing artifact, cancellation, or unconfigured real store publishing.
- Added `cli.PublishRelease` to enforce `publishing.allow_publish`, acquire the publish lock, hash the existing artifact, create a publish ledger run, and update `.panex/state.json`.
- Registered MCP `publish_release` with `target`, `artifact_path`, `profile_ref`, and `dry_run` arguments.
- Added tests for tool registration, default policy denial, allowed dry-run publishing, publish ledger persistence, Chrome dry-run success, and missing profile rejection.
- Removed the deferred publish MCP gap from the spec gap inventory.

## Risk and Mitigation

- Risk: accidental external publishing. Mitigation: the Chrome adapter only supports dry-run success; real store publishing returns a structured `blocked` result until a backend is explicitly configured in a future PR.
- Risk: publishing without approval. Mitigation: `cli.PublishRelease` evaluates `publishing.allow_publish` before acquiring a publish lock or touching artifacts.
- Risk: publishing stale or absent artifacts. Mitigation: the tool hashes and validates an existing artifact path, or uses the newest target artifact only when one exists.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./internal/target ./internal/mcp -run 'TestChrome_PublishArtifact|TestToolsList|TestToolPublishRelease' -count=1`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` (failed in the linked worktree because Go could not obtain VCS status; rerun with VCS stamping disabled)
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build` (failed in the linked worktree because Go could not obtain VCS status; rerun with VCS stamping disabled)
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Teach-Back

Publish is a separate release-plane operation from packaging. The first implementation should make policy, profile, artifact, lock, and ledger boundaries real before adding store-specific upload backends.

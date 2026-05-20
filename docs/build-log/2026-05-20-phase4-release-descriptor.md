# Phase 4 Release Descriptor

**Status:** implemented
**Date:** 2026-05-20

## What

Package runs now emit a self-contained `release.json` descriptor plus `artifacts.json` under `.panex/runs/<run-id>/`.

## Why

The release descriptor is the stable bridge between packaging, future publishing, CI integrations, and Insights. Packaging already produced target artifacts, but there was no public descriptor that aggregated project identity, target manifests, permissions, capability states, artifact digests, and verification state.

## Approach

- Added `internal/releasedesc` with schema-versioned descriptor types, a builder, and stable JSON writing.
- Added `docs/release-descriptor.schema.json` so the public descriptor shape is machine-readable.
- Updated `CmdPackage` to compile target manifests, compute verification summary, package artifacts, write `release.json`, write `artifacts.json`, and include descriptor data in JSON output.
- Recorded descriptor paths in the package run ledger evidence.
- Added descriptor package tests and CLI package-run coverage for emitted release evidence.

## Risk and Mitigation

- Risk: packaging could start depending on future publish behavior. Mitigation: the descriptor keeps `publish_metadata` explicit and null; publishing remains a separate operation.
- Risk: downstream consumers could rely on unstable ad hoc fields. Mitigation: the schema version and JSON Schema document the current public contract.
- Risk: verification hard blocks could make legacy package tests fail before artifact creation. Mitigation: package records the verification summary in the descriptor but does not use it as a new package gate in this PR.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./internal/releasedesc ./internal/cli -run 'TestBuildDescriptor|TestWriteFile|TestCmdPackage|TestCmdReport_AfterPackage|TestFullWorkflow' -count=1`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` failed in the linked worktree because Go VCS stamping could not resolve repository status.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make build` failed in the linked worktree because Go VCS stamping could not resolve repository status.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Teach-Back

Release evidence should be written at the package boundary, before publish exists. That gives later publish code a concrete artifact contract instead of reconstructing state from graph, run, and adapter internals.

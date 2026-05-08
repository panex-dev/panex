# Phase 1 cross-platform tail fixes

**Status:** PR pending
**Date:** 2026-05-08

## Problem

After the broader Phase 1 cross-platform cleanup landed through later PRs, this branch still carried two real behavior fixes that had not yet reached `main`:

1. `cmd/panex/doctor.go` still let `isWSL()` consult the stubbed `/proc/version` reader on non-Linux platforms. That made the WSL warning path testable on Windows in ways that do not reflect any real runtime and left the helper semantically broader than the platform it is meant to detect.
2. `internal/daemon/file_watcher.go` already skipped infrastructure directories when registering watches, but Windows can still surface parent-directory events for excluded paths through `ReadDirectoryChangesW`. Those events were entering the pending queue, so ignored directories like `.git` could still look like watched changes.

## Approach

- `cmd/panex/doctor.go` now returns early from `isWSL()` unless `runtime.GOOS == "linux"`. WSL detection is a Linux-only concern; the helper should not infer Linux-specific behavior from test stubs or foreign platforms.
- `internal/daemon/file_watcher.go` now rejects infrastructure paths before queuing them into `pending`, and keeps the normalization-time guard that rejects events whose first path segment is an infrastructure directory. This preserves the existing watch-registration boundary while also filtering backend-specific stray events.

## Risk and mitigation

- Risk: the watcher filter could hide a legitimate top-level file that merely resembles an infrastructure name.
- Mitigation: the check operates on normalized path segments and matches the same infrastructure-directory contract already used by `addDirectoryTree`; regular files like `.gitignore` remain valid while descendants under `.git/`, `.panex/`, and similar directories stay excluded.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Push the rebased branch, watch the full GitHub Actions matrix, and merge once every required check is green on the final head commit.

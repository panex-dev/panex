# PR6 - File Watcher with Debounced Internal Events

## Metadata
- Date: 2026-03-01
- PR: 6
- Branch: `feat/file-watcher`
- Title: add filesystem watcher with 50ms debouncing

## Problem
- The daemon had no way to detect local source changes, so nothing could trigger build/reload work.
- Raw filesystem events are noisy and can flood downstream steps when editors write multiple times per save.

## Approach (with file+line citations)
- Added a daemon-level watcher with explicit constructor validation:
  - `internal/daemon/file_watcher.go:28-56`
- Implemented recursive directory watch registration, including newly created directories:
  - `internal/daemon/file_watcher.go:132-146`
  - `internal/daemon/file_watcher.go:152-166`
- Added 50ms debounced batching and path deduplication:
  - `internal/daemon/file_watcher.go:19`
  - `internal/daemon/file_watcher.go:73-114`
  - `internal/daemon/file_watcher.go:120-149`
- Standardized path shape for cross-platform downstream consumers:
  - `internal/daemon/file_watcher.go:169-185`
- Added tests for validation, debouncing, new-directory watch behavior, and cancel-flush semantics:
  - `internal/daemon/file_watcher_test.go:12-225`
- Recorded strategy and reversibility in ADR-005:
  - `docs/adr/005-file-watching-strategy.md:1-25`

## Risk and Mitigation
- Risk: platform-specific event quirks can produce duplicate events.
- Mitigation: per-window path deduplication plus sorted deterministic payloads before emit.

## Verification
- Commands:
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Expected:
  - watcher unit tests pass
  - no lint errors
  - binary build remains green

## Teach-back (engineering lessons)
- Debounce is a correctness primitive, not only a performance optimization, when file writes fan out.
- Relative path emission prevents platform-coupled behavior from leaking into protocol contracts.
- Watch-tree maintenance (new directory handling) must be explicit; most watcher libraries do not recurse automatically.

## Next Step
- Integrate watcher events into build orchestration and emit `build.complete` over WebSocket after successful bundling.

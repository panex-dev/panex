# PR7 - Esbuild Integration and build.complete Emission

## Metadata
- Date: 2026-03-01
- PR: 7
- Branch: `feat/esbuild-integration`
- Title: integrate esbuild bundling and emit build.complete on watcher events

## Problem
- File changes were detected, but there was no bundling step and no event broadcast to consumers.
- Without a build result signal, the dev agent cannot reliably decide when to reload.

## Approach (with file+line citations)
- Added in-process esbuild builder with result struct and deterministic entry discovery:
  - `internal/build/esbuild_builder.go:17-188`
- Added builder tests for validation, success path, syntax failure, missing entries, and canceled context:
  - `internal/build/esbuild_builder_test.go:12-195`
- Added WebSocket server broadcast API for protocol envelopes:
  - `internal/daemon/websocket_server.go:221-255`
- Added broadcast integration test after handshake:
  - `internal/daemon/websocket_server_test.go:103-158`
- Wired daemon runtime loop in CLI:
  - configure builder + watcher in `cmd/panex/main.go:125-148`
  - run concurrent server/watcher/build loops in `cmd/panex/main.go:150-174`
  - emit `build.complete` in `cmd/panex/main.go:177-234`
- Recorded architecture decision:
  - `docs/adr/006-esbuild-integration.md:1-25`

## Risk and Mitigation
- Risk: full rebuild per change can become expensive as projects grow.
- Mitigation: this keeps semantics simple in MVP; later move to incremental build contexts without changing protocol consumers.

## Verification
- Commands:
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Expected:
  - build package tests pass
  - daemon broadcast tests pass
  - binary build remains green

## Teach-back (engineering lessons)
- Event emission should happen at the boundary where state transitions complete (`build finished`) to keep clients simple.
- In-process tool embedding reduces orchestration failure modes and simplifies deterministic builds.
- Designing around stable result contracts (`build.complete`) enables backend strategy changes later.

## Next Step
- Implement Dev Agent handling of `command.reload` after successful `build.complete` to complete the visible save->reload loop.

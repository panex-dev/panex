# PR65 - Daemon Session Lifecycle

## Metadata
- Date: 2026-03-09
- PR: 65
- Branch: `fix/pr65-daemon-session-lifecycle`
- Title: harden daemon session lifecycle cancellation and close serialization
- Commit(s): `fix(daemon): harden websocket session lifecycle`

## Problem
- The audit still had two websocket server correctness issues in the daemon: client message handling in `readLoop` used `context.Background()`, and failed broadcast cleanup closed connections outside the session write lock.
- That meant in-flight store-backed client commands could ignore daemon shutdown, and the session close path was split across multiple call sites with inconsistent synchronization.

## Approach (with file+line citations)
- Change 1:
  - Why: add a server-owned lifecycle context that is canceled on `Close()` so in-flight client command handling can stop when the daemon shuts down.
  - Where: `internal/daemon/websocket_server.go:56-84`
  - Where: `internal/daemon/websocket_server.go:105-140`
  - Where: `internal/daemon/websocket_server.go:201-208`
  - Where: `internal/daemon/websocket_server.go:318-340`
- Change 2:
  - Why: centralize per-session writes, control frames, and closes behind one session lock so broadcast failure cleanup and shutdown use the same synchronization path.
  - Where: `internal/daemon/websocket_server.go:343-378`
  - Where: `internal/daemon/websocket_server.go:1253-1318`
- Change 3:
  - Why: add regression tests for both stale-session broadcast cleanup and cancellation of an in-flight `query.events` request on daemon close.
  - Where: `internal/daemon/websocket_server_test.go:182-284`
  - Where: `internal/daemon/websocket_server_test.go:1501-1589`
  - Where: `internal/daemon/websocket_server_test.go:1706-1724`
- Change 4:
  - Why: record the resolved audit item in the tracker and build status log.
  - Where: `audit.md:5-24`
  - Where: `docs/build-log/STATUS.md:61-66`
  - Where: `docs/build-log/2026-03-09-pr-065-daemon-session-lifecycle.md:1-43`

## Risk and mitigation
- Risk: canceling the daemon lifecycle context from `Close()` could change cleanup behavior for tests or callers that previously expected active websocket requests to keep running while shutdown started.
- Mitigation: the new regression test pins the intended shutdown behavior, and the server already treats shutdown as a terminal path for active connections.
- Risk: moving close operations behind the session lock could deadlock if a close happens while a write still holds the mutex.
- Mitigation: writes and closes are now funneled through the same tiny session helpers, and the full daemon test package passes with the new helpers in place.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -run "WebSocketBroadcastUnregistersClosedSession|WebSocketCloseCancelsInFlightQueryEvents" -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1`
- Additional checks:
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
  - Result: full repo verification completed cleanly after the daemon-specific regressions passed.

## Teach-back
- Design lesson: websocket request handling needs a server-owned lifecycle context; request-scoped or background contexts are both wrong once the handler upgrades and returns.
- Testing lesson: shutdown semantics need explicit regression tests with controllable blocking dependencies, otherwise cancellation bugs only show up under teardown races.
- Workflow lesson: when a concurrency bug spans multiple call sites, centralize the primitive first and then route all call sites through it in the same PR.

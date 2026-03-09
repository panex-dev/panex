# PR74 - Daemon WebSocket Deadlines

## Metadata
- Date: 2026-03-09
- PR: 74
- Branch: `fix/pr74-daemon-websocket-deadlines`
- Title: add daemon websocket read/write deadlines and ping keepalive
- Commit(s):
  - `fix(daemon): add websocket read and write deadlines`
  - `docs: record daemon websocket deadline hardening`

## Problem
- The daemon only applied a handshake read deadline, so established websocket sessions could stall indefinitely if a client stopped responding or a write blocked forever.
- That left the transport vulnerable to slow or dead peers holding resources long after the handshake succeeded.

## Approach (with file+line citations)
- Change 1:
  - Why: add configurable per-session read and write deadlines, plus a ping keepalive loop, so established sessions are bounded after the handshake instead of relying on open-ended socket behavior.
  - Where: `internal/daemon/websocket_server.go:23-52`
  - Where: `internal/daemon/websocket_server.go:95-126`
  - Where: `internal/daemon/websocket_server.go:214-375`
  - Where: `internal/daemon/websocket_server.go:1341-1374`
- Change 2:
  - Why: route session writes and control frames through deadline-aware helpers so broadcast, close, and per-session writes all share the same transport bound.
  - Where: `internal/daemon/websocket_server.go:385-409`
  - Where: `internal/daemon/websocket_server.go:1341-1364`
- Change 3:
  - Why: add regressions for both sides of the keepalive contract: responsive clients stay connected, and clients that stop answering ping are removed after the read timeout.
  - Where: `internal/daemon/websocket_server_test.go:325-378`
  - Where: `internal/daemon/websocket_server_test.go:1603-1667`
- Change 4:
  - Why: move the roadmap to the next supportability slice once websocket deadline hardening is complete.
  - Where: `docs/build-log/STATUS.md:73-86`
  - Where: `docs/build-log/README.md:44-50`
  - Where: `docs/build-log/2026-03-09-pr-074-daemon-websocket-deadlines.md:1-40`

## Risk and Mitigation
- Risk: aggressive deadlines could disconnect healthy clients if keepalive timing is too tight.
- Mitigation: the server keeps conservative defaults, refreshes read deadlines on both pong and inbound messages, and tests a short-deadline configuration explicitly before relying on it in production defaults.
- Risk: adding a ping loop could introduce more close/unregister races around already-closing sessions.
- Mitigation: the ping loop exits on the server context or the session close channel, and session writes/controls stay serialized through the existing session lock.

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
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -run "WebSocketPingKeepsResponsiveClientConnected|WebSocketClosesClientThatStopsRespondingToPing" -count=1`
  - The focused keepalive test rerun required unsandboxed networking because `httptest` attempted an IPv6 listener inside the sandbox.

## Teach-back
- Design lesson: a websocket handshake timeout is not enough; established sessions need their own liveness contract or dead peers become resource leaks.
- Testing lesson: keepalive tests need to model client read behavior correctly, because gorilla clients only auto-respond to ping while a read loop is active.
- Team workflow lesson: when sandbox limits invalidate a real transport test, rerun the same test outside the sandbox instead of substituting a weaker check.

## Next Step
- Add optional agent diagnostic logging for websocket lifecycle and command handling to improve extension-side debugging without widening default noise.

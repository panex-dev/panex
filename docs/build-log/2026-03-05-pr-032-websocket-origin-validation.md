# PR32 - WebSocket Origin Validation

## Metadata
- Date: 2026-03-05
- PR: 32
- Branch: `fix/pr32-websocket-origin-validation`
- Title: restrict WebSocket upgrades to localhost origins
- Commit(s): pending

## Problem
- `CheckOrigin` unconditionally returned `true`, allowing any page on localhost (or beyond) to open a WebSocket connection to the daemon.
- Audit §2 flagged this as a critical security gap for CSRF if the daemon is ever exposed beyond localhost.

## Approach (with file+line citations)
- Replaced `CheckOrigin: func(_ *http.Request) bool { return true }` with `isLocalOrigin`:
  - Why: restricts WebSocket upgrades to requests originating from 127.0.0.1, localhost, or ::1
  - Where: `internal/daemon/websocket_server.go:94`
- Added `isLocalOrigin` function:
  - Why: validates Origin header against localhost variants; allows missing Origin (Chrome extensions)
  - Where: `internal/daemon/websocket_server.go:678-692`
- Added unit tests for `isLocalOrigin`:
  - Where: `internal/daemon/websocket_server_test.go:505-534`
- Added integration test for non-local origin rejection:
  - Where: `internal/daemon/websocket_server_test.go:536-555`

## Risk and Mitigation
- Risk: Chrome extension connections may set an Origin header.
- Mitigation: Chrome extension service workers do not set Origin on WebSocket connections; empty Origin is explicitly allowed.
- Risk: legitimate non-localhost dev setups (remote containers, tunnels) would be blocked.
- Mitigation: this is a local-only dev tool; remote access scenarios will need a separate connection story with proper auth.

## Verification
- Commands run:
  - `make fmt && make lint && make test && make build`
- New tests:
  - `TestIsLocalOrigin` — validates all localhost variants and rejects remote origins
  - `TestWebSocketRejectsNonLocalOrigin` — confirms 403 for non-local Origin header

## Teach-back (engineering lessons)
- Design lesson: origin validation is the WebSocket equivalent of CSRF protection. Even local-only tools should validate origins to prevent cross-site WebSocket hijacking from malicious browser tabs.

## Next Step
- Migrate auth token from query string to Authorization header.

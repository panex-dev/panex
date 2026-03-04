# PR33 - Constant-Time Token Comparison

## Metadata
- Date: 2026-03-05
- PR: 33
- Branch: `fix/pr33-auth-header-migration`
- Title: use constant-time comparison for WebSocket auth token
- Commit(s): pending

## Problem
- Token comparison used `==` which is vulnerable to timing attacks — an attacker could infer token characters by measuring response time.
- Audit §2 recommended migrating to Authorization header, but the browser WebSocket API does not support custom headers. The query parameter approach is the standard pattern for WebSocket auth in browsers.

## Approach (with file+line citations)
- Replaced `==` with `crypto/subtle.ConstantTimeCompare` for token validation:
  - Why: eliminates timing side-channel in token comparison
  - Where: `internal/daemon/websocket_server.go:200-201`
- Added documentation comment explaining why query parameter is necessary:
  - Why: future maintainers should understand this is a WebSocket API limitation, not an oversight
  - Where: `internal/daemon/websocket_server.go:198-199`

## Risk and Mitigation
- Risk: `ConstantTimeCompare` returns 1 for equal, 0 for not equal — off-by-one risk if comparison is inverted.
- Mitigation: explicit `== 1` check matches the standard Go idiom for constant-time comparison.
- Risk: token remains in URL query string (visible in logs/history).
- Mitigation: unavoidable for browser WebSocket connections. The daemon is localhost-only, and token auth is defense-in-depth behind origin validation.

## Verification
- Commands run:
  - `make fmt && make lint && make test && make build`
- Existing auth tests (`TestWebSocketAuthRejectsMissingToken`) continue to pass.

## Teach-back (engineering lessons)
- Design lesson: not all audit recommendations are implementable. The browser WebSocket API cannot set custom headers — the standard approach is query parameters or subprotocols. Document limitations rather than forcing an impossible migration.
- Security lesson: constant-time comparison is cheap to add and eliminates an entire class of side-channel attacks. Always use `subtle.ConstantTimeCompare` for secret comparison.

## Next Step
- Add integration test suite that exercises the daemon → agent → inspector path.

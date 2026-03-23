# ADR-004: WebSocket Handshake and Token Auth for Dev Sessions

## Status
Accepted — amended 2026-03-23 (auth moved from query parameter to hello payload)

## Context
Panex components communicate over WebSocket, and the MVP needs authentication plus a deterministic first-message handshake.
Browser extensions cannot attach arbitrary HTTP headers in a WebSocket constructor, which makes header-based bearer auth impractical for the Dev Agent.

## Decision
- Expose a single endpoint: `GET /ws` with no query parameters.
- Accept the WebSocket upgrade unconditionally (no HTTP-level auth).
- Require the first frame from clients to be a `hello` message (`lifecycle` type) containing `protocol_version`, `auth_token`, `client_kind`, `client_version`, and optionally `extension_id` and `capabilities_requested`.
- Validate `auth_token` in the `hello` payload using constant-time comparison.
- Respond with `hello.ack` carrying `session_id`, `daemon_version`, `auth_ok`, and `capabilities_supported`.
- If `auth_token` is missing or invalid, respond with `hello.ack` where `auth_ok = false` and close the connection.
- Close the connection with policy-violation close code for invalid first message or malformed handshake payload.

## Consequences
- Auth is compatible with browser extension constraints — no headers or query parameters needed.
- The token never appears in URLs or server access logs, reducing accidental exposure.
- Session tracking starts immediately after successful handshake.
- Future hardening can migrate to subprotocol-based auth or ephemeral signed tokens.

## Reversibility
Medium.
The handshake semantics are versioned by protocol envelope (`v`), so we can introduce an alternate auth mechanism in `v=2` while still supporting `v=1` during migration.

## Amendment Log
- **2026-03-23:** Auth was originally specified as a `token` query parameter during WebSocket upgrade (`/ws?token=<auth_token>`). The implementation instead authenticates via the `auth_token` field in the `hello` message payload (see `internal/daemon/websocket_server.go`). This ADR has been updated to reflect the actual message-payload auth approach, which avoids token leakage in URLs and server logs.

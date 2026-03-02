# ADR-004: WebSocket Handshake and Token Auth for Dev Sessions

## Status
Accepted

## Context
Panex components communicate over WebSocket, and the MVP needs authentication plus a deterministic first-message handshake.
Browser extensions cannot attach arbitrary HTTP headers in a WebSocket constructor, which makes header-based bearer auth impractical for the Dev Agent.

## Decision
- Expose a single endpoint: `GET /ws`.
- Require a `token` query parameter during upgrade (`/ws?token=<auth_token>`).
- Require the first frame from clients to be `hello` (`lifecycle` type) with `protocol_version = 1`.
- Respond with `hello.ack` carrying `session_id`, `daemon_version`, `auth_ok`, and `capabilities_supported`.
- Close the connection with policy-violation close code for invalid first message or malformed handshake payload.

## Consequences
- Auth is compatible with browser extension constraints in MVP.
- Session tracking starts immediately after successful handshake.
- Token can appear in logs if request URLs are logged; operators should avoid logging raw query strings.
- Future hardening can migrate to subprotocol-based auth or ephemeral signed tokens.

## Reversibility
Medium.
The handshake semantics are versioned by protocol envelope (`v`), so we can introduce an alternate auth mechanism in `v=2` while still supporting `v=1` during migration.

# PR68 - WebSocket Hello Auth

## Metadata
- Date: 2026-03-09
- PR: 68
- Branch: `fix/pr68-websocket-hello-auth`
- Title: move daemon auth from websocket query params into the hello handshake
- Commit(s):
  - `fix(websocket): move daemon auth into hello handshake`

## Problem
- The audit still had an open auth hardening gap: browser clients were carrying the daemon token in the websocket URL query string as `?token=...`.
- That leaked credentials into URL surfaces and logs, and kept the auth contract split between the HTTP upgrade and the protocol handshake.

## Approach (with file+line citations)
- Change 1:
  - Why: move the token onto the protocol handshake itself and keep the Go/TypeScript protocol definitions aligned.
  - Where: `internal/protocol/types.go:92-99`
  - Where: `shared/protocol/src/index.ts:62-69`
- Change 2:
  - Why: let the daemon accept the websocket upgrade, require `hello.data.auth_token`, send `hello.ack` with `auth_ok=false` on auth failure, and bound the unauthenticated wait with a handshake read deadline.
  - Where: `internal/daemon/websocket_server.go:25-28`
  - Where: `internal/daemon/websocket_server.go:214-326`
- Change 3:
  - Why: remove token transport from browser websocket URLs while still delivering auth through the first hello message.
  - Where: `agent/src/background.ts:22-46`
  - Where: `agent/src/config.ts:26-29`
  - Where: `agent/src/config.ts:41-66`
  - Where: `inspector/src/connection.ts:76-78`
  - Where: `inspector/src/connection.ts:159-190`
  - Where: `inspector/src/connection.ts:499-523`
  - Where: `shared/chrome-sim/src/transport.ts:56-60`
  - Where: `shared/chrome-sim/src/transport.ts:171-217`
  - Where: `shared/chrome-sim/src/transport.ts:420-423`
- Change 4:
  - Why: replace the old URL-token tests with handshake-auth coverage and prove stale `token=` params are stripped from saved/browser websocket URLs.
  - Where: `internal/daemon/websocket_server_test.go:18-78`
  - Where: `internal/daemon/websocket_server_test.go:129-170`
  - Where: `internal/daemon/websocket_server_test.go:1547-1662`
  - Where: `internal/daemon/integration_test.go:45-118`
  - Where: `internal/daemon/integration_test.go:232-263`
  - Where: `agent/tests/config.test.ts:60-100`
  - Where: `inspector/tests/connection.test.ts:14-35`
  - Where: `inspector/tests/workbench.test.ts:61-85`
  - Where: `shared/chrome-sim/tests/transport.test.ts:30-72`
  - Where: `shared/chrome-sim/tests/transport.test.ts:206-233`
- Change 5:
  - Why: record the resolved audit item and keep the documentation consistent with the new contract.
  - Where: `audit.md:34-43`
  - Where: `docs/build-log/STATUS.md:64-69`
  - Where: `inspector/README.md:28-40`
  - Where: `shared/chrome-sim/README.md:15-20`

## Protocol parity impact (required when protocol changes)
- [x] This PR updates Go protocol definitions (`internal/protocol/types.go`) when required.
- [x] This PR updates TypeScript protocol definitions (`shared/protocol/src/index.ts`) when required.
- [x] `go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1` passes.
- [x] I described protocol compatibility impact (additive vs breaking) in this PR.

Compatibility impact: additive and backward-compatible at the message schema level. `hello` now carries optional `auth_token`, and the daemon now requires it for authentication instead of reading `?token=` from the websocket URL.

## Risk and mitigation
- Risk: removing pre-upgrade auth could let unauthenticated clients hold open idle sockets longer than before.
- Mitigation: the daemon now applies a 5-second handshake read deadline before any client is authenticated.
- Risk: stored or user-supplied websocket URLs could still include stale `token=` params and silently keep leaking them.
- Mitigation: every browser-side websocket URL builder strips `token` query params before connect, and unit tests pin that cleanup.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1`
  - `pnpm --dir shared/protocol test`
  - `pnpm --dir agent test`
  - `pnpm --dir inspector test`
  - `pnpm --dir shared/chrome-sim test`
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`

## Teach-back
- Design lesson: browser websocket auth should live in the first protocol message when the transport cannot supply real headers; splitting auth across URL state and protocol state is a liability.
- Testing lesson: auth-contract migrations need both positive-path integration coverage and explicit failure-path tests, or stale compatibility assumptions linger in helpers and fixtures.
- Workflow lesson: when removing sensitive data from URLs, also strip old query params from every normalization path, not just the current happy-path constructor.

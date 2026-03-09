# PR67 - Browser WebSocket Message Guard

## Metadata
- Date: 2026-03-09
- PR: 67
- Branch: `fix/pr67-browser-websocket-message-guard`
- Title: guard browser websocket clients against oversized inbound frames
- Commit(s):
  - `fix(websocket): guard browser-side inbound message size`

## Problem
- The audit still had an open browser-side websocket receive-limit gap: the daemon capped inbound websocket frames at 1 MiB, but the browser clients would still attempt to decode arbitrarily large inbound frames.
- That left the agent, inspector, and `chrome-sim` transport relying on browser defaults instead of enforcing the same message-size contract as the daemon.

## Approach (with file+line citations)
- Change 1:
  - Why: add a shared TypeScript helper that normalizes websocket message payloads and reports oversize frames against the 1 MiB contract.
  - Where: `shared/protocol/src/index.ts:1-24`
  - Where: `shared/protocol/src/index.ts:225-247`
- Change 2:
  - Why: route all three browser-side websocket consumers through that helper and close sockets with code `1009` before decoding oversized frames.
  - Where: `agent/src/background.ts:1-69`
  - Where: `inspector/src/connection.ts:1-254`
  - Where: `shared/chrome-sim/src/transport.ts:1-267`
- Change 3:
  - Why: add direct helper tests plus a `chrome-sim` transport regression that proves an oversized inbound frame gets closed instead of decoded.
  - Where: `shared/protocol/tests/websocket.test.ts:1-45`
  - Where: `shared/chrome-sim/tests/transport.test.ts:8-246`
- Change 4:
  - Why: mark the audit item resolved and record the slice in the build log.
  - Where: `audit.md:26-38`
  - Where: `docs/build-log/STATUS.md:63-68`
  - Where: `docs/build-log/2026-03-09-pr-067-browser-websocket-message-guard.md:1-37`

## Protocol parity impact (required when protocol changes)
- [x] This PR updates Go protocol definitions (`internal/protocol/types.go`) when required.
- [x] This PR updates TypeScript protocol definitions (`shared/protocol/src/index.ts`) when required.
- [x] `go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1` passes.
- [x] I described protocol compatibility impact (additive vs breaking) in this PR.

Compatibility impact: non-breaking. The wire protocol is unchanged; the browser clients now enforce the existing 1 MiB websocket frame contract before decode.

## Risk and mitigation
- Risk: a shared websocket helper in `shared/protocol` could blur the line between protocol definitions and transport behavior.
- Mitigation: the helper only codifies the existing frame-size contract and message-data normalization that all browser clients already needed; it does not introduce client-specific policy.
- Risk: closing on oversized messages could surface as more aggressive reconnect behavior in browser clients.
- Mitigation: the close path uses websocket code `1009` consistently, and the `chrome-sim` regression test pins that oversized frames are rejected before decode.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
  - `pnpm --dir shared/protocol test`
  - `pnpm --dir shared/chrome-sim test`
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`

## Teach-back
- Design lesson: if multiple browser transports need the same safety contract, put the byte-level guard in the shared layer instead of re-deriving it in each client.
- Testing lesson: transport hardening is easiest to prove where sockets are already faked, then let shared helpers carry that contract into thinner consumers.
- Workflow lesson: when a browser API cannot enforce a limit natively, enforce the contract at the first byte-normalization boundary rather than at higher-level decode code.

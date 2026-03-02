# PR16 - Protocol hello.ack Capability Negotiation

## Metadata
- Date: 2026-03-02
- PR: 16
- Branch: `feat/protocol-hello-ack-cap-negotiation`
- Title: switch handshake to hello.ack and add capability negotiation
- Commit(s): pending

## Problem
- The handshake used `welcome`, which did not support explicit capability negotiation between daemon and clients.
- Foundation work requires protocol-level capability awareness so clients can detect available features safely at connect time.

## Approach (with file+line citations)
- Replaced `welcome` with `hello.ack` in Go and TypeScript protocol contracts, and expanded handshake payloads:
  - `internal/protocol/types.go:21-169`
  - `shared/protocol/src/index.ts:9-154`
- Added daemon-side capability negotiation and returned `capabilities_supported` in `hello.ack`:
  - `internal/daemon/websocket_server.go:21-458`
- Updated inspector and agent handshake payloads to send client identity + requested capabilities:
  - `inspector/src/main.tsx:1-180`
  - `agent/src/background.ts:20-66`
- Updated handshake/protocol tests to cover `hello.ack` and capability negotiation behavior:
  - `internal/daemon/websocket_server_test.go:36-425`
  - `internal/protocol/types_test.go:9-263`
  - `internal/protocol/codec_test.go:7-58`
  - `shared/protocol/tests/index.test.ts:1-81`
- Updated ADRs to reflect current handshake contract:
  - `docs/adr/003-protocol-envelope.md:10-13`
  - `docs/adr/004-websocket-handshake-auth.md:11-15`
  - `docs/adr/010-inspector-timeline-architecture.md:11-14`
  - `docs/adr/014-shared-typescript-protocol-module.md:15`

## Risk and Mitigation
- Risk: older clients may still send legacy `capabilities`.
- Mitigation: daemon falls back from `capabilities_requested` to `capabilities` during handshake parsing (`internal/daemon/websocket_server.go:227-231`).
- Risk: handshake rename could break inspector bootstrap.
- Mitigation: inspector explicitly waits for `hello.ack` and has test coverage in protocol guard suites.

## Verification
- Commands run:
  - `go test ./...`
  - `cd agent && npm run check && npm test`
  - `cd inspector && npm run check && npm test && node --import tsx --test ../shared/protocol/tests/*.test.ts`
- Notes:
  - Go tests required normal cache access in this environment; reran with elevated permissions and all packages passed.

## Teach-back (engineering lessons)
- Handshake messages should encode compatibility contracts, not just transport readiness.
- Capability negotiation belongs in protocol payloads, not inferred from client type or guessed server version.
- Backward-compatible decode paths reduce migration risk while still letting us evolve protocol semantics.

## Next Step
- Add automated drift checks that fail CI when Go and TypeScript protocol message names/payload keys diverge.

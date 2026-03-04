# PR34 - Integration Test Suite

## Metadata
- Date: 2026-03-05
- PR: 34
- Branch: `test/pr34-integration-tests`
- Title: add daemon lifecycle integration test
- Commit(s): pending

## Problem
- Every test was a unit test. No test exercised the full daemon → agent → inspector path.
- Audit §3 flagged this as the biggest testing gap: individual pieces work but the system was never tested as a system.

## Approach (with file+line citations)
- Added `TestIntegrationDaemonLifecycle` that exercises the complete lifecycle:
  - Agent connects and handshakes with capability negotiation
  - Daemon broadcasts build.complete + command.reload to agent
  - Inspector connects and handshakes
  - Inspector queries events, verifies persisted timeline contains hello, hello.ack, build.complete, command.reload
  - Daemon mutates storage, both clients receive storage.diff
  - Inspector queries storage, verifies snapshot reflects mutation
  - Both clients disconnect cleanly
  - Where: `internal/daemon/integration_test.go:28-233`
- Added focused test helpers (`dial`, `handshake`, `readEnvelope`) separate from existing unit test helpers:
  - Why: integration helpers have different semantics (read deadlines, simpler signatures)
  - Where: `internal/daemon/integration_test.go:237-275`

## Risk and Mitigation
- Risk: integration test depends on event ordering which could vary under load.
- Mitigation: test uses sequential operations (handshake → broadcast → query) with read deadlines, not concurrent flows.
- Risk: test is slower than unit tests due to full WebSocket round-trips.
- Mitigation: single test exercises all paths in ~100ms; acceptable for CI.

## Verification
- Commands run:
  - `go test -race -count=1 ./internal/daemon/ -run TestIntegration`
  - `make fmt && make lint && make test && make build`

## Teach-back (engineering lessons)
- Design lesson: integration tests catch category errors that unit tests miss — serialization round-trips, event ordering, multi-session broadcast correctness. One good integration test is worth dozens of mocked unit tests for system confidence.
- Testing lesson: integration tests should exercise the happy path end-to-end with assertions at each boundary. Error-path testing stays in unit tests.

## Next Step
- All audit items resolved. Resume transport wiring for storage simulation mutation calls.

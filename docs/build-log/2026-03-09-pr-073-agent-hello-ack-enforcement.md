# PR73 - Dev-Agent Hello Ack Enforcement

## Metadata
- Date: 2026-03-09
- PR: 73
- Branch: `feat/pr73-agent-hello-ack-enforcement`
- Title: enforce dev-agent `hello.ack` completion before accepting live commands
- Commit(s):
  - `fix(agent): require hello.ack before reload commands`
  - `docs: record dev-agent hello.ack enforcement`

## Problem
- The dev-agent sent `hello` on websocket open, but it accepted any valid envelope immediately afterward.
- That meant `command.reload` could run before the handshake was actually acknowledged, which made capability negotiation advisory instead of real behavior.

## Approach (with file+line citations)
- Change 1:
  - Why: centralize the dev-agent handshake contract in one testable module that builds `hello`, tracks handshake completion, rejects pre-ack live traffic, and requires negotiated `command.reload` capability before reload commands are honored.
  - Where: `agent/src/handshake.ts:1-83`
  - Where: `agent/tests/handshake.test.ts:1-185`
- Change 2:
  - Why: make the background worker consume that handshake module so reconnects reset handshake state and stale sockets do not drive command handling after replacement.
  - Where: `agent/src/background.ts:1-89`
- Change 3:
  - Why: advance the roadmap to the next websocket hardening slice once the agent-side handshake follow-on is no longer open.
  - Where: `docs/build-log/STATUS.md:73-85`
  - Where: `docs/build-log/README.md:44-51`
  - Where: `docs/build-log/2026-03-09-pr-073-agent-hello-ack-enforcement.md:1-38`

## Risk and Mitigation
- Risk: stricter handshake enforcement could close connections in cases that the old agent tolerated silently.
- Mitigation: the stricter behavior is limited to protocol-invalid or negotiation-failed states, and the reconnect path remains unchanged for valid development sessions.
- Risk: handshake state could leak across reconnects and make later sessions appear ready prematurely.
- Mitigation: the background worker resets handshake state on socket open and close, and ignores stale socket events before they can affect the current session.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm --dir agent check`
  - `pnpm --dir agent test`
  - `pnpm --dir agent build`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - `agent/tests/handshake.test.ts` verifies pre-ack command rejection, failed handshake rejection, missing `command.reload` negotiation, and post-ack reload acceptance.

## Teach-back
- Design lesson: a handshake only matters if post-connect behavior is actually gated on it; otherwise capability negotiation is documentation, not contract.
- Testing lesson: protocol hardening work is easiest to keep honest when the state machine is extracted into a small pure module instead of being buried inside websocket side effects.
- Team workflow lesson: once a follow-on hardening item is closed, move the roadmap immediately so the next runtime-risk slice stays explicit.

## Next Step
- Add daemon websocket read/write deadlines so slow or stalled clients cannot hold server resources indefinitely.

# PR36 - Simulator Transport Protocol Messages (`chrome.api.call/result/event`)

## Metadata
- Date: 2026-03-05
- PR: 36
- Branch: `feat/pr36-simulator-transport-protocol`
- Title: add chrome simulator transport envelope family to shared Go/TS protocol contracts
- Commit(s): pending

## Problem
- The roadmap sequence after PR35 requires a protocol-level contract for simulator RPC traffic before daemon/router/shim implementation can proceed.
- Without explicit `chrome.api.*` envelope names and payload models, transport correlation (`call_id`) and parity checks cannot be implemented safely across Go and TS.

## Approach (with file+line citations)
- Added new protocol message names and payload models in Go:
  - Message names:
    - `internal/protocol/types.go:21-38`
  - Payload models:
    - `internal/protocol/types.go:181-199`
  - Message-type mapping and constructors:
    - `internal/protocol/types.go:201-218`
    - `internal/protocol/types.go:277-287`
- Added matching message names/mappings/interfaces in TypeScript:
  - Envelope names + type mapping:
    - `shared/protocol/src/index.ts:9-46`
  - Payload interfaces:
    - `shared/protocol/src/index.ts:150-168`
- Extended parity/contract tests so Go <-> TS drift is still enforced:
  - Parity list update:
    - `internal/protocol/parity_test.go:41-58`
  - Go message-type and constructor tests:
    - `internal/protocol/types_test.go:16-33`
    - `internal/protocol/types_test.go:317-350`
  - TS message mapping assertions:
    - `shared/protocol/tests/index.test.ts:94-98`

## Risk and Mitigation
- Risk: adding new message names in one language but not the other could silently break runtime decoding.
- Mitigation: parity tests assert full envelope-name equality and message-type-map equality (`internal/protocol/parity_test.go:41-70`).
- Risk: early payload shape decisions might block simulator implementation.
- Mitigation: payloads are intentionally transport-focused and minimal (`call_id`, namespace/method, args, success/data/error), leaving handler semantics for the next PR.

## Verification
- Commands run:
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -count=1`
  - `cd shared/protocol && npm run check`
  - `cd shared/protocol && npm test`
- Expected:
  - Go protocol tests pass with new `chrome.api.*` message coverage.
  - TS protocol check/tests pass with updated envelope mapping and interfaces.

## Teach-back (engineering lessons)
- Protocol-first sequencing continues to de-risk simulator work; transport contracts are testable before behavior is implemented.
- Drift gates stay valuable as protocol surface area grows; they prevent accidental one-sided additions.

## Next Step
- Implement daemon-side simulator router for `chrome.api.call` focused on storage operations (`storage.local/sync`: `get/set/remove/clear/getBytesInUse`) and emit correlated `chrome.api.result`.

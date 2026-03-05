# PR35 - Storage Mutation Transport Wiring (Inspector -> WebSocket -> Daemon APIs)

## Metadata
- Date: 2026-03-05
- PR: 35
- Branch: `feat/pr35-storage-mutation-transport-wiring`
- Title: wire inspector storage simulation mutations to daemon storage APIs via websocket commands
- Commit(s): pending

## Problem
- The storage subsystem could query snapshots (`query.storage`) and render diffs (`storage.diff`), but inspector interactions could not trigger real storage mutations over transport.
- Daemon mutation APIs (`SetStorageItem`, `RemoveStorageItem`, `ClearStorageArea`) existed but were only callable in-process, leaving the simulation loop incomplete.

## Approach (with file+line citations)
- Extended shared protocol contract with mutation command messages and payload models:
  - Added `storage.set`, `storage.remove`, `storage.clear` names, payload structs/interfaces, type mappings, and constructors in Go:
    - `internal/protocol/types.go:21-35`
    - `internal/protocol/types.go:163-249`
  - Added matching message names/mappings/interfaces in TypeScript:
    - `shared/protocol/src/index.ts:9-40`
    - `shared/protocol/src/index.ts:129-142`
- Kept protocol parity and constructor coverage strict:
  - Updated parity guard expected message list:
    - `internal/protocol/parity_test.go:41-57`
  - Added Go tests for message lookup + constructors:
    - `internal/protocol/types_test.go:16-30`
    - `internal/protocol/types_test.go:285-313`
  - Added TS test assertions for new command mappings:
    - `shared/protocol/tests/index.test.ts:88-92`
- Wired daemon websocket command handling to existing mutation APIs:
  - Advertised new capabilities:
    - `internal/daemon/websocket_server.go:26-34`
  - Added command handlers for `storage.set/remove/clear` in `handleClientMessage` and routed to mutation methods:
    - `internal/daemon/websocket_server.go:344-477`
- Wired inspector transport and storage tab mutation UI:
  - Added connection context methods + command builders with area/key normalization:
    - `inspector/src/connection.ts:45-57`
    - `inspector/src/connection.ts:94-134`
    - `inspector/src/connection.ts:355-472`
  - Added storage tab mutation controls (set/remove/clear), value parsing, and error messaging:
    - `inspector/src/tabs/storage.ts:13-246`
  - Wired tab props and styling updates:
    - `inspector/src/main.tsx:36-44`
    - `inspector/src/styles.css:214-232`
- Added daemon websocket transport tests for the new command path:
  - End-to-end command -> diff -> query state checks for set/remove/clear:
    - `internal/daemon/websocket_server_test.go:446-678`
  - Invalid payload rejection coverage:
    - `internal/daemon/websocket_server_test.go:701-741`

## Risk and Mitigation
- Risk: malformed mutation commands could poison daemon state or create silent no-op behavior.
- Mitigation: daemon reuses strict area/key validation in existing mutation APIs and closes websocket on policy violations (`internal/daemon/websocket_server.go:440-472`, `internal/daemon/websocket_server_test.go:701-741`).
- Risk: inspector could emit invalid mutation payloads from user input.
- Mitigation: client-side normalization rejects invalid area/key before send and surfaces explicit UI errors (`inspector/src/connection.ts:367-472`, `inspector/src/tabs/storage.ts:154-203`).

## Verification
- Commands run:
  - `go test ./internal/protocol ./internal/daemon -count=1`
  - `cd shared/protocol && npm test`
  - `cd shared/protocol && npm run check`
  - `cd inspector && npm test`
  - `cd inspector && npm run check`
- Expected:
  - Go protocol + daemon suites pass, including new websocket storage mutation command tests.
  - Shared protocol tests/check pass with no TS/Go drift.
  - Inspector tests/check pass with new mutation wiring.

## Teach-back (engineering lessons)
- Exposed daemon mutation APIs made transport wiring low-risk because state logic did not need to be duplicated in websocket handlers.
- Protocol parity tests are the right guardrail when adding cross-language command families; they catch drift immediately.
- Wiring simulation UX as a thin client over protocol commands keeps inspector behavior honest and debuggable in the event timeline.

## Next Step
- Implement the simulator call/result protocol family (`chrome.api.call`, `chrome.api.result`, `chrome.api.event`) as the next sequencing milestone toward full chrome-sim transport.

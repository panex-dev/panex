# PR23 - Storage Protocol Extension and Daemon Stub Handler

## Metadata
- Date: 2026-03-04
- PR: 23
- Branch: `feat/pr23-storage-protocol`
- Title: add query.storage/query.storage.result/storage.diff protocol and daemon stub
- Commit(s): pending

## Problem
- The Storage tab scaffold existed, but there was no protocol contract to request storage state.
- Without storage query messages, PR24 storage UI would need to invent ad hoc data paths and could not stay protocol-driven.

## Approach (with file+line citations)
- Extended Go protocol message names, payload models, message-type mapping, and constructors for storage queries/diffs:
  - `internal/protocol/types.go:21-226`
- Extended TypeScript shared protocol envelope names, mapping, payload interfaces, and result guard:
  - `shared/protocol/src/index.ts:9-188`
- Added daemon capability advertisement for `query.storage`:
  - `internal/daemon/websocket_server.go:23-27`
- Added daemon command handler path for `query.storage` returning empty snapshots (all areas by default, single area when requested) with area validation:
  - `internal/daemon/websocket_server.go:326-415`
  - `internal/daemon/websocket_server.go:463-481`
- Added daemon integration tests for storage query success and invalid-area rejection:
  - `internal/daemon/websocket_server_test.go:298-385`
- Updated protocol tests/parity checks to include new message names and constructors:
  - `internal/protocol/types_test.go:9-302`
  - `internal/protocol/parity_test.go:41-64`
  - `shared/protocol/tests/index.test.ts:4-87`
- Added ADR documenting storage protocol decision and stub semantics:
  - `docs/adr/017-storage-protocol-extension.md:1-36`

## Risk and Mitigation
- Risk: introducing protocol names in one language only would create drift.
- Mitigation: Go+TS contracts were updated together and parity checks were updated (`internal/protocol/parity_test.go:41-64`).
- Risk: early storage semantics could be ambiguous for unsupported areas.
- Mitigation: daemon validates area values (`local`, `sync`, `session`) and rejects unsupported inputs deterministically (`internal/daemon/websocket_server.go:473-476`).

## Verification
- Commands run:
  - `go test ./internal/protocol -count=1`
  - `go test ./internal/daemon -count=1`
  - `go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
  - `go test ./...`
  - `pnpm --dir shared/protocol install --no-frozen-lockfile`
  - `pnpm --dir shared/protocol run check`
  - `pnpm --dir shared/protocol run test`
- Expected:
  - Protocol and daemon tests pass with storage query coverage.
  - Cross-language parity test passes with new message names.
  - Shared TS protocol check/tests pass with new interfaces/guard.

## Teach-back (engineering lessons)
- Schema-first delivery is effective when UI work depends on stable contracts; stub handlers are enough to unblock downstream tabs.
- Protocol capabilities should evolve with handlers so clients can negotiate features rather than infer them from versions.
- Additive protocol increments are safest when parity and constructor tests are updated in the same change.

## Next Step
- Implement PR24 storage viewer UI using `query.storage.result` snapshots and prepare for future `storage.diff` event rendering.

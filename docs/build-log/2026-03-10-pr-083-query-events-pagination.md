# PR83 - Query Events Pagination

## Metadata
- Date: 2026-03-10
- PR: 83
- Branch: `feat/pr83-query-events-pagination`
- Title: paginate persisted timeline history and expose a Timeline "load older" flow
- Commit(s):
  - `feat(timeline): paginate query.events history loading`

## Problem
- The inspector always asked the daemon for one fixed recent-history page and then trimmed subsequent merges back to that same cap.
- That kept the first Timeline view fast, but it made older persisted history unreachable and blocked the first real timeline-scalability increment from the queued follow-ons.

## Approach (with file+line citations)
- Change 1:
  - Why: extend the protocol and SQLite event store with an honest pagination contract so the daemon can return older history without guessing whether more rows remain.
  - Where: `internal/protocol/types.go:129-143`
  - Where: `shared/protocol/src/index.ts:99-113`
  - Where: `internal/store/sqlite_event_store.go:96-149`
- Change 2:
  - Why: thread the new `before_id` cursor and `has_more` response through the daemon query handler and cover both recent-history and paginated older-history behavior with websocket tests.
  - Where: `internal/daemon/websocket_server.go:58-61`
  - Where: `internal/daemon/websocket_server.go:463-501`
  - Where: `internal/daemon/websocket_server_test.go:475-596`
- Change 3:
  - Why: let the inspector retain each loaded history page, request older pages on demand, and present a minimal Timeline "load older" control instead of silently trimming loaded history back to the initial page size.
  - Where: `inspector/src/timeline.ts:21-93`
  - Where: `inspector/src/connection.ts:53-117`
  - Where: `inspector/src/connection.ts:188-270`
  - Where: `inspector/src/connection.ts:366-403`
  - Where: `inspector/src/main.tsx:29-77`
  - Where: `inspector/src/tabs/timeline.tsx:12-140`
- Change 4:
  - Why: add regression coverage for prepend-style timeline merges, cursor discovery, and inspector query construction so the new pagination path is protected without requiring UI harness complexity.
  - Where: `inspector/tests/timeline.test.ts:44-83`
  - Where: `inspector/tests/timeline.test.ts:209-224`
  - Where: `inspector/tests/connection.test.ts:52-71`
  - Where: `internal/store/sqlite_event_store_test.go:18-149`

## Risk and Mitigation
- Risk: pagination could reorder persisted history or silently duplicate rows across page boundaries.
- Mitigation: the store over-fetches one extra row to compute `has_more`, returns results in chronological order, and the inspector timeline tests now cover prepend merges and id-based deduplication.
- Risk: the inspector could still drop loaded history when live traffic resumes.
- Mitigation: the connection layer now grows retained timeline capacity page by page and reuses that capacity for both older-page merges and live appends.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `GOCACHE=/tmp/go-build go test ./internal/store ./internal/daemon ./internal/protocol -count=1`
  - `pnpm --dir inspector test`
  - `pnpm --dir shared/protocol test`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Verified the daemon pagination tests under unrestricted networking because websocket listener setup is not valid inside the sandboxed network namespace.

## Teach-back
- Design lesson: timeline scalability is safer as a protocol/store contract first and a UI affordance second; once the backend can answer honest paginated queries, the inspector change stays small.
- Testing lesson: pagination tests have to reflect the full persisted stream, including handshake history, or they will accidentally encode a fake storage model.
- Workflow lesson: when a roadmap item is intentionally broad, the right PR is the first end-to-end slice that closes one contract boundary, not a speculative refactor around it.

## Next Step
- Select the next post-release milestone from the remaining queued follow-ons.

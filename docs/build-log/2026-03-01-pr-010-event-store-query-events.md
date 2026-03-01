# PR10 - SQLite Event Store and query.events Endpoint

## Metadata
- Date: 2026-03-01
- PR: 10
- Branch: `feat/event-store`
- Title: persist protocol events in sqlite and expose query.events
- Commit(s): pending

## Problem
- Protocol activity existed only in-memory and was lost across daemon restarts.
- There was no client-facing endpoint to retrieve historical protocol activity for inspector/timeline use.

## Approach (with file+line citations)
- Extended protocol contract with request/response types for historical queries:
  - added message names, payloads, and constructors in `internal/protocol/types.go:21-168`
  - added protocol mapping/constructor tests in `internal/protocol/types_test.go:16-263`
- Added SQLite-backed event store package:
  - WAL-mode schema setup and append/recent APIs in `internal/store/sqlite_event_store.go:18-154`
  - store validation, ordering, and limit behavior tests in `internal/store/sqlite_event_store_test.go:11-114`
- Wired event-store path into config and runtime bootstrap:
  - default `server.event_store_path` handling in `internal/config/config.go:13-65`
  - config default/override tests in `internal/config/config_test.go:10-64`
  - runtime pass-through into daemon server config in `cmd/panex/main.go:127-134`
- Integrated store lifecycle and query endpoint into WebSocket server:
  - store setup + server close semantics in `internal/daemon/websocket_server.go:56-154`
  - persistence on handshake/broadcast paths in `internal/daemon/websocket_server.go:187-309`
  - `query.events` handling and response writes in `internal/daemon/websocket_server.go:311-378`
  - endpoint behavior test coverage in `internal/daemon/websocket_server_test.go:175-247`
- Added SQLite driver dependency required by the new store:
  - `modernc.org/sqlite` and transitive modules in `go.mod:5-26`, `go.sum`

## Risk and Mitigation
- Risk: persistence failures could silently degrade protocol observability.
- Mitigation: broadcast and handshake paths now fail fast with contextual errors when persistence cannot complete (`internal/daemon/websocket_server.go:218-244`, `internal/daemon/websocket_server.go:271-274`).
- Risk: query endpoints can return non-deterministic ordering if database access is naive.
- Mitigation: store query scans newest-first for efficiency, then reverses results for chronological consumption (`internal/store/sqlite_event_store.go:99-138`).

## Verification
- Commands:
  - `go test ./internal/protocol ./internal/store ./internal/daemon ./internal/config ./cmd/panex`
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Expected:
  - new store/query tests pass
  - full repository quality gates remain green

## Teach-back (engineering lessons)
- Persistence boundaries should be attached where protocol envelopes are emitted, not reconstructed later from logs.
- Query/read APIs should encode ordering semantics explicitly so clients do not infer timeline rules ad hoc.
- Storage-backed features should arrive with dedicated unit tests before endpoint wiring to keep debugging scope narrow.

## Next Step
- Build inspector scaffold on top of `query.events.result` to render live and historical timeline slices from one protocol channel.

# Build Status Tracker

As of 2026-03-04.

## Completed
| PR | Scope | Evidence |
|---|---|---|
| 1-5 | Foundation setup sequence | `docs/build-log/2026-02-28-pr-001-005-foundation.md` |
| 6 | File watcher + debounce | `docs/build-log/2026-03-01-pr-006-file-watcher.md` |
| 7 | Esbuild + `build.complete` emission | `docs/build-log/2026-03-01-pr-007-esbuild-build-complete.md` |
| 8 | Dev Agent scaffold + `command.reload` handling | `docs/build-log/2026-03-01-pr-008-dev-agent-scaffold.md` |
| 9 | Daemon reload command emission | `docs/build-log/2026-03-01-pr-009-extension-reload.md` |
| 10 | SQLite event store + `query.events` | `docs/build-log/2026-03-01-pr-010-event-store-query-events.md` |
| 11 | Inspector scaffold | `docs/build-log/2026-03-01-pr-011-inspector-scaffold.md` |
| 12 | Inspector filters/search | `docs/build-log/2026-03-01-pr-012-inspector-filters.md` |
| 13 | Query operators + persistence | `docs/build-log/2026-03-01-pr-013-inspector-query-persistence.md` |
| 14 | Inspector resilience/hardening | `docs/build-log/2026-03-01-pr-014-inspector-resilience.md` |
| 15 | Shared TS protocol module | `docs/build-log/2026-03-01-pr-015-shared-protocol-module.md` |
| 16 | TypeScript CI across TS packages | `docs/build-log/2026-03-04-pr-016-typescript-ci.md` |
| 17 | `hello.ack` + capability negotiation | `docs/build-log/2026-03-02-pr-017-hello-ack-capability-negotiation.md` |
| 18 | Go/TS protocol drift check | `docs/build-log/2026-03-02-pr-018-protocol-drift-check.md` |
| 19 | PR template parity gate + status tracker | `docs/build-log/2026-03-04-pr-019-pr-template-protocol-parity.md` |
| 20 | Numbering reconciliation (plan vs build-log sequence) | `docs/build-log/2026-03-04-pr-020-numbering-reconciliation.md` |
| 21 | Unified inspector shell + hash router decomposition | `docs/build-log/2026-03-04-pr-021-unified-inspector-shell.md` |
| 22 | Sidebar component extraction | `docs/build-log/2026-03-04-pr-022-sidebar-component.md` |
| 23 | Storage protocol extension + daemon stub handler | `docs/build-log/2026-03-04-pr-023-storage-protocol-extension.md` |
| 24 | Storage viewer UI with `query.storage.result` area filtering | `docs/build-log/2026-03-04-pr-024-storage-viewer-ui.md` |
| 25 | Storage diff ingestion + row highlight in inspector storage tab | `docs/build-log/2026-03-04-pr-025-storage-diff-highlights.md` |
| 26 | Daemon storage mutation pipeline + `storage.diff` fanout | `docs/build-log/2026-03-04-pr-026-daemon-storage-diff-pipeline.md` |
| 27 | Branch hygiene workflow + CI guardrails | (process PR, no build-log entry) |
| 28 | Audit cleanup: validation, error handling, debuggability | `docs/build-log/2026-03-04-pr-028-audit-cleanup.md` |
| 29 | Inspector Content-Security-Policy | `docs/build-log/2026-03-04-pr-029-inspector-csp.md` |
| 30 | Replace test time.Sleep with polling+deadline | `docs/build-log/2026-03-04-pr-030-test-sleep-polling.md` |
| 31 | Thread context.Context through store calls | `docs/build-log/2026-03-04-pr-031-store-context-threading.md` |

## In progress
- None.

## Next
- Implement transport wiring from inspector storage simulation into daemon mutation APIs via WebSocket.

## Notes
- PR20 is intentionally reserved as documentation reconciliation so sequence alignment is explicit and auditable.

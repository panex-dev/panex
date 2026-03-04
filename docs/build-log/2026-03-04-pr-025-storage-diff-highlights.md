# PR25 - Storage Diff Ingestion and Row Highlighting

## Metadata
- Date: 2026-03-04
- PR: 25
- Branch: `feat/pr25-storage-diff-highlights`
- Title: ingest `storage.diff` events and highlight changed storage rows
- Commit(s): pending

## Problem
- PR24 rendered storage snapshots, but live `storage.diff` events were not applied to inspector state.
- Without diff ingestion, the Storage tab stayed stale between manual refreshes and could not indicate which keys changed.

## Approach (with file+line citations)
- Extended connection context to track storage highlight state, request `storage.diff` capability, and apply incoming `storage.diff` payloads while preserving timeline ingestion:
  - `inspector/src/connection.ts:42-51`
  - `inspector/src/connection.ts:56-86`
  - `inspector/src/connection.ts:105-119`
  - `inspector/src/connection.ts:163-177`
  - `inspector/src/connection.ts:272-302`
  - `inspector/src/connection.ts:356-357`
- Added storage diff application helpers that mutate snapshots by area/key, generate stable row IDs, and return changed-row sets for UI highlighting:
  - `inspector/src/storage.ts:10-20`
  - `inspector/src/storage.ts:57-141`
  - `inspector/src/storage.ts:166-181`
- Wired highlight state into Storage tab row rendering and added highlight styling:
  - `inspector/src/main.tsx:35-41`
  - `inspector/src/tabs/storage.ts:12-17`
  - `inspector/src/tabs/storage.ts:45-50`
  - `inspector/src/styles.css:237-239`
- Expanded storage unit coverage for diff apply semantics and row IDs:
  - `inspector/tests/storage.test.ts:58-89`
  - `inspector/tests/storage.test.ts:41-48`
- Advanced build-log tracker/status to PR25 completion and PR26 next target:
  - `docs/build-log/STATUS.md:27-33`
  - `docs/build-log/README.md:44-46`

## Risk and Mitigation
- Risk: malformed or partial diff payloads could corrupt storage state.
- Mitigation: diff application validates area/changes/key shape and ignores malformed entries (`inspector/src/storage.ts:90-118`).
- Risk: highlight state could drift from actual rows if diff keys are duplicated or reordered.
- Mitigation: changed row IDs are deduplicated and mapped through deterministic `area:key` IDs (`inspector/src/storage.ts:126-141`, `inspector/src/storage.ts:170-181`).

## Verification
- Commands run:
  - `pnpm --dir inspector run check`
  - `pnpm --dir inspector run test`
  - `pnpm --dir inspector run build`
- Expected:
  - TypeScript check passes for inspector.
  - Storage diff tests pass with existing inspector suite.
  - Build succeeds with new highlight rendering path.

## Teach-back (engineering lessons)
- Snapshot UIs need an incremental update path (`diff`) in the same increment, otherwise manual refresh becomes an operational dependency.
- Stable row IDs (`area:key`) reduce UI-state complexity and make highlight logic/testing predictable.
- Keeping connection parsing and state mutation in small helper functions makes protocol-event expansion cheaper and safer.

## Next Step
- Implement PR26 daemon-side storage mutation pipeline that emits `storage.diff` events from real storage operations.

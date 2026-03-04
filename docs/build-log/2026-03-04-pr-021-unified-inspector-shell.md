# PR21 - Unified Inspector Shell, Router, and Connection Context

## Metadata
- Date: 2026-03-04
- PR: 21
- Branch: `feat/pr21-unified-inspector-shell`
- Title: decompose inspector main into shell, router, and connection provider
- Commit(s): pending

## Problem
- `inspector/src/main.tsx` held transport lifecycle, timeline state, filtering, and rendering in one file.
- That coupling blocked the next milestone (multi-tab shell) and increased regression risk for future Storage/Workbench/Replay work.

## Approach (with file+line citations)
- Extracted websocket lifecycle + handshake/query bootstrap + reconnect + shared timeline state into a connection context provider:
  - `inspector/src/connection.ts:1-259`
- Added hash-based tab routing for `#timeline`, `#storage`, `#workbench`, and `#replay`:
  - `inspector/src/router.ts:1-59`
- Added shell component with tab bar, sidebar slot, and content slot:
  - `inspector/src/shell.ts:1-52`
- Moved timeline-specific filter/search/persistence rendering into a dedicated tab module:
  - `inspector/src/tabs/timeline.ts:1-195`
- Added storage placeholder tab and disabled-state tab rendering for workbench/replay:
  - `inspector/src/tabs/storage.ts:1-14`
  - `inspector/src/main.tsx:18-80`
- Reduced `main.tsx` to composition only (provider + shell + router + tab selection + boundary):
  - `inspector/src/main.tsx:1-105`
- Updated styles for shell layout, tab controls, sidebar layout, and placeholders while preserving timeline card/filter styling:
  - `inspector/src/styles.css:17-279`
- Recorded architecture decision for unified shell decomposition:
  - `docs/adr/016-unified-inspector-shell.md:1-38`

## Risk and Mitigation
- Risk: router + tab decomposition could change existing timeline behavior.
- Mitigation: timeline merge/filter logic and reconnect utilities remain unchanged (`inspector/src/timeline.ts`, `inspector/src/reconnect.ts`), and timeline UI was moved with equivalent controls in `tabs/timeline.ts`.
- Risk: introducing context/provider indirection can hide transport failures.
- Mitigation: connection status and error state are surfaced in the shell sidebar (`inspector/src/main.tsx:29-43`).

## Verification
- Commands run:
  - `pnpm --dir inspector run check`
  - `pnpm --dir inspector run test`
  - `pnpm --dir inspector run build`
- Expected:
  - Type-check passes.
  - Existing reconnect/timeline tests pass.
  - Inspector bundle builds successfully.

## Teach-back (engineering lessons)
- Multi-surface UI work should split shared runtime state from tab-local presentation before feature expansion.
- Hash routing is a low-cost way to support direct links and browser back/forward semantics in local tools.
- Keeping timeline semantics in one shared module made the shell refactor mostly structural, not behavioral.

## Next Step
- Implement PR22 sidebar component extraction and shared sidebar actions so tab content remains focused on domain-specific UI.

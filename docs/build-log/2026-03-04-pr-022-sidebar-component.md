# PR22 - Sidebar Component Extraction

## Metadata
- Date: 2026-03-04
- PR: 22
- Branch: `feat/pr22-sidebar-component`
- Title: extract shared sidebar component from inspector main shell
- Commit(s): pending

## Problem
- Sidebar rendering still lived inline in `inspector/src/main.tsx`, which kept shell composition coupled to connection presentation details.
- Upcoming tabs should reuse one shared sidebar contract, not duplicate inline markup.

## Approach (with file+line citations)
- Added a dedicated sidebar component with stable props for connection status, daemon URL, and error display, plus placeholder action buttons:
  - `inspector/src/sidebar.ts:1-32`
- Rewired shell composition in `main.tsx` to consume the sidebar component instead of inline sidebar markup:
  - `inspector/src/main.tsx:7-57`
- Kept responsive two-column shell/sidebar styles from PR21 as the visual contract for extracted sidebar usage:
  - `inspector/src/styles.css:47-85`

## Risk and Mitigation
- Risk: extraction could change connection/error rendering behavior.
- Mitigation: sidebar props map directly to existing `connection` accessors (`status`, `socketURL`, `lastError`) and retain identical labels/actions.
- Risk: shell composition drift across tabs.
- Mitigation: sidebar remains injected through `Shell` slot API from one place (`main.tsx`).

## Verification
- Commands run:
  - `pnpm --dir inspector run check`
  - `pnpm --dir inspector run test`
  - `pnpm --dir inspector run build`
- Expected:
  - Type-check passes.
  - Existing inspector tests pass.
  - Inspector bundle builds successfully.

## Teach-back (engineering lessons)
- Extracting slot content into dedicated components keeps shell contracts explicit and scales better than inline markup in the app root.
- Passing accessors directly preserves reactivity without introducing additional context layers.
- Small UI extractions are safer when done immediately after shell decomposition, before more tabs land.

## Next Step
- Implement PR23 storage protocol extension (`query.storage`, `query.storage.result`, `storage.diff`) with daemon stubs and shared protocol updates.

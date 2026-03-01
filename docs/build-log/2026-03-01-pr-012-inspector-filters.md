# PR12 - Inspector Timeline Filters and Search

## Metadata
- Date: 2026-03-01
- PR: 12
- Branch: `feat/inspector-filters`
- Title: add inspector timeline filtering and search controls
- Commit(s): pending

## Problem
- Timeline volume grows quickly once historical hydration and live updates are both active.
- Without local filtering, inspecting specific command/event flows requires manual scanning.

## Approach (with file+line citations)
- Added reusable filter model and filtering utility in timeline layer:
  - filter types/defaults in `inspector/src/timeline.ts:10-18`
  - filtering implementation in `inspector/src/timeline.ts:90-117`
- Wired filter state into inspector runtime and display:
  - local filter signals and filtered timeline computation in `inspector/src/main.tsx:40-50`
  - filter controls (search/type/source) in `inspector/src/main.tsx:157-206`
  - filtered rendering/counts in `inspector/src/main.tsx:141-176`
- Added UI styles for filter controls across desktop/mobile:
  - `inspector/src/styles.css:63-112`
- Extended timeline tests for filter behavior:
  - `inspector/tests/timeline.test.ts:79-136`
- Updated inspector usage docs:
  - `inspector/README.md:15-16`
- Recorded architecture decision:
  - `docs/adr/011-inspector-filtering-model.md:1-24`

## Risk and Mitigation
- Risk: inconsistent behavior if filtering occurs before dedupe/ordering.
- Mitigation: keep filters applied after normalized merge path in `inspector/src/main.tsx:45-50`.
- Risk: search behavior can be too narrow if only raw payload is indexed.
- Mitigation: index message metadata plus summary text in `inspector/src/timeline.ts:104-111`.

## Verification
- Commands:
  - `cd inspector && pnpm run check`
  - `cd inspector && pnpm run test`
  - `cd inspector && pnpm run build`
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Expected:
  - filter utility tests pass
  - inspector bundle stays green
  - full repo quality gates remain green

## Teach-back (engineering lessons)
- Keep transport ingestion and display filtering separate to avoid coupling protocol correctness with UI concerns.
- Filter semantics should be implemented in a reusable utility layer, then surfaced via UI controls.
- Timeline features need test coverage for both positive matches and narrowing behavior to avoid regression under event scale.

## Next Step
- Add persisted inspector preferences and richer query operators (e.g., `name:`, `src:`, `build:`) for faster debugging workflows.

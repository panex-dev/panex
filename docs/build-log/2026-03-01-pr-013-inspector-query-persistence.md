# PR13 - Inspector Query Operators and Filter Persistence

## Metadata
- Date: 2026-03-01
- PR: 13
- Branch: `feat/inspector-query-operators`
- Title: add inspector query operators and persisted filter preferences
- Commit(s): pending

## Problem
- Free-text filtering alone is slow when timeline volume is high and events are semantically similar.
- Reloading inspector clears active filters, forcing repeated manual setup during debugging loops.

## Approach (with file+line citations)
- Extended timeline model with query clauses and parser:
  - `TimelineFilter` defaults and query clause model in `inspector/src/timeline.ts:10-21`
  - token parser + operator support in `inspector/src/timeline.ts:159-188`
  - clause-aware filtering in `inspector/src/timeline.ts:103-157`
- Wired query operators + persisted preferences into UI runtime:
  - filter state initialization and filtered timeline computation in `inspector/src/main.tsx:37-54`
  - localStorage load/save + guards in `inspector/src/main.tsx:313-345`
  - enhanced controls and reset action in `inspector/src/main.tsx:178-229`
- Added styling for hint text and reset control:
  - `inspector/src/styles.css:63-128`
- Added tests for parser/operator behavior:
  - `inspector/tests/timeline.test.ts:80-182`
- Updated inspector docs for operator usage:
  - `inspector/README.md:15-21`
- Captured decision rationale:
  - `docs/adr/012-inspector-query-and-preferences.md:1-25`

## Risk and Mitigation
- Risk: query behavior can become ambiguous as syntax grows.
- Mitigation: keep parser strict, token-based, and covered by deterministic unit tests (`inspector/tests/timeline.test.ts:139-182`).
- Risk: localStorage failures in private/locked-down environments could break UI behavior.
- Mitigation: persistence path is wrapped in safe try/catch fallbacks (`inspector/src/main.tsx:307-331`).

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
  - parser and filter tests pass
  - inspector build emits updated bundle and styles
  - repository quality gates remain green

## Teach-back (engineering lessons)
- Query operator support belongs in the utility layer, not ad hoc in UI callbacks.
- Persisting workflow state has outsized DX impact when iterating on event-heavy systems.
- Small, explicit grammars plus tests beat ambitious parsers early in product evolution.

## Next Step
- Introduce optional server-side filtered history queries when timeline size exceeds local memory thresholds.

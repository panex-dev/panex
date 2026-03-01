# PR11 - Inspector Scaffold with Live Event Timeline

## Metadata
- Date: 2026-03-01
- PR: 11
- Branch: `feat/inspector-scaffold`
- Title: scaffold SolidJS inspector with websocket timeline
- Commit(s): pending

## Problem
- Protocol history and live messages existed, but there was no visual interface to inspect event flow.
- Without a timeline UI, debugging build/reload behavior still required raw logs and manual message tracing.

## Approach (with file+line citations)
- Added a dedicated inspector package with deterministic scripts and dependencies:
  - `inspector/package.json:2-21`
  - `inspector/pnpm-lock.yaml:1-400`
  - `inspector/tsconfig.json:2-15`
- Added browser build pipeline and static entrypoint:
  - `inspector/scripts/build.mjs:1-21`
  - `inspector/index.html:1-11`
- Implemented protocol models and guards for inspector transport handling:
  - `inspector/src/protocol.ts:1-69`
- Implemented timeline normalization/merge logic for historical snapshots + live events:
  - `inspector/src/timeline.ts:3-85`
- Implemented Solid inspector runtime:
  - websocket connect + hello in `inspector/src/main.tsx:57-79`
  - welcome-triggered `query.events` in `inspector/src/main.tsx:96-106`
  - query hydration + live merge in `inspector/src/main.tsx:108-204`
  - timeline rendering and tail-follow behavior in `inspector/src/main.tsx:44-177`
- Added timeline utility tests:
  - `inspector/tests/timeline.test.ts:20-71`
- Added inspector package docs + repo references:
  - `inspector/README.md:1-25`
  - `README.md:27-30`
  - `.gitignore:30-35`
- Captured architecture decision for inspector flow:
  - `docs/adr/010-inspector-timeline-architecture.md:1-25`

## Risk and Mitigation
- Risk: timeline duplication or ordering drift when combining query snapshots and live stream.
- Mitigation: dedupe by snapshot id and cap list size in `inspector/src/timeline.ts:32-64`.
- Risk: malformed messages could break rendering state.
- Mitigation: strict envelope guards and query payload checks in `inspector/src/protocol.ts:46-69`, `inspector/src/main.tsx:86-195`.

## Verification
- Commands:
  - `cd inspector && pnpm install`
  - `cd inspector && pnpm run check`
  - `cd inspector && pnpm run test`
  - `cd inspector && pnpm run build`
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Expected:
  - inspector bundle emits `dist/main.js` and `dist/main.css`
  - timeline tests pass
  - repository quality gates remain green

## Teach-back (engineering lessons)
- Inspector UIs should consume protocol contracts directly to avoid creating second-order backend APIs too early.
- Hydration-first (`query.events`) plus streaming (`Broadcast`) gives deterministic startup without sacrificing real-time behavior.
- Small utility-level tests around merge/dedupe logic prevent subtle timeline regressions later.

## Next Step
- Add inspector filters/search and context panes so timeline becomes actionable beyond chronological browsing.

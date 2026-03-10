# PR88 - Timeline History Scalability

## Metadata
- Date: 2026-03-11
- PR: 88
- Branch: `feat/pr88-timeline-history-scalability`
- Title: bound the inspector Timeline to a reloadable working set so older browsing does not grow client memory without limit
- Commit(s):
  - `feat(timeline): bound inspector history working set`

## Problem
- PR84 capped only the rendered DOM tail. The inspector still kept every paged-in Timeline event in one client array, so `load older` increased retained memory and filter/search work without any upper bound.
- Once an operator paged backward far enough, the client needed a truthful way to get back to the newest tail without pretending the trimmed recent page was still in memory.

## Approach (with file+line citations)
- Change 1:
  - Why: define an explicit Timeline working-set cap and make merge helpers report whether overflow trimmed the oldest or newest side, so the client can bound retained history without hiding which edge was discarded.
  - Where: `inspector/src/timeline.ts:28-36`
  - Where: `inspector/src/timeline.ts:63-129`
- Change 2:
  - Why: keep the inspector Timeline bounded in the connection layer, track trimmed older/newer counts, and re-query the latest persisted page when the operator jumps back to newest after older browsing shifted the retained window.
  - Where: `inspector/src/connection.ts:54-68`
  - Where: `inspector/src/connection.ts:79-149`
  - Where: `inspector/src/connection.ts:301-367`
  - Where: `inspector/src/connection.ts:451-515`
- Change 3:
  - Why: surface the bounded working-set state in the Timeline header so operators can see when older history was trimmed locally or newer history is available only via a fresh latest-page query, and keep the build-log tracker aligned with the new roadmap state.
  - Where: `inspector/src/main.tsx:32-45`
  - Where: `inspector/src/main.tsx:72-85`
  - Where: `inspector/src/tabs/timeline.tsx:16-145`
  - Where: `inspector/tests/timeline.test.ts:48-121`
  - Where: `docs/build-log/STATUS.md:84-95`
  - Where: `docs/build-log/README.md:44-45`

## Risk and Mitigation
- Risk: bounding the retained Timeline working set could make the inspector silently lose context after long sessions.
- Mitigation: the Timeline header now labels the active window explicitly and reports trimmed older/newer counts so the operator can see when history rolled out of the client.
- Risk: reloading the newest page after older browsing could drop live events received while that latest query is in flight.
- Mitigation: the connection layer buffers live envelopes during a latest-page reset and merges them back after the query result is applied.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm --dir inspector test`
  - `pnpm --dir inspector check`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`

## Teach-back
- Design lesson: render windowing is only half of Timeline scalability; if the connection layer still accumulates every page, filters and summary tabs keep paying the full unbounded cost.
- State-management lesson: once a bounded client window can shift away from the newest tail, “jump to newest” needs to be an explicit data reload contract, not just a scroll reset.
- Workflow lesson: fresh PR worktrees need their own dependency bootstrap before TypeScript checks mean anything, so record the install command as part of verification instead of assuming a warm environment.

## Next Step
- Add multi-extension support.

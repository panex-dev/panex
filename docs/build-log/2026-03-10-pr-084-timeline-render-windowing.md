# PR84 - Timeline Render Windowing

## Metadata
- Date: 2026-03-10
- PR: 84
- Branch: `feat/pr84-timeline-render-windowing`
- Title: cap default Timeline rendering to a live tail window with explicit older-history reveal controls
- Commit(s):
  - `feat(timeline): cap default inspector render window`

## Problem
- PR83 made older persisted timeline history reachable, but the inspector still filtered and rendered the entire loaded list on every update.
- That kept the protocol honest while leaving the UI scalability follow-on open: loading more history immediately grew the DOM and the old auto-scroll effect kept snapping the operator back to the newest tail.

## Approach (with file+line citations)
- Change 1:
  - Why: define a small default render window and pure helpers for slicing the loaded tail plus counting hidden older entries, so the scalability boundary is explicit and testable outside the component.
  - Where: `inspector/src/timeline.ts:28-34`
  - Where: `inspector/src/timeline.ts:104-121`
- Change 2:
  - Why: keep the Timeline tab anchored to the newest tail by default, add explicit controls to reveal older loaded entries or collapse back to the newest window, and stop auto-scrolling when the operator has expanded older history.
  - Where: `inspector/src/tabs/timeline.tsx:25-127`
  - Where: `inspector/src/tabs/timeline.tsx:186-205`
  - Where: `inspector/src/styles.css:145-176`
- Change 3:
  - Why: protect the new render-window contract with focused tests and advance the build-log tracker now that one deeper-timeline-scalability slice is closed.
  - Where: `inspector/tests/timeline.test.ts:89-128`
  - Where: `docs/build-log/STATUS.md:84-94`
  - Where: `docs/build-log/README.md:44-48`

## Risk and Mitigation
- Risk: a smaller default render window could hide already-loaded history and make the Timeline feel incomplete.
- Mitigation: the header now shows rendered vs filtered vs loaded counts, exposes a dedicated `show older loaded` control, and keeps the daemon-backed `load older` action separate.
- Risk: removing unconditional auto-scroll could make the live tail harder to follow.
- Mitigation: the Timeline still follows the newest tail by default and exposes an explicit `jump to newest` action after the operator expands older history.

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
- Additional checks:
  - Confirmed the fresh PR worktree started from `origin/main` at `e7c7b5fc93cb62e00c347d2e921ce2c58c97a889`.
  - Reran `CI=1 pnpm install --frozen-lockfile` and `GOCACHE=/tmp/go-build make test` outside the sandbox when dependency downloads and websocket-backed tests needed unrestricted networking.

## Teach-back
- Design lesson: once pagination makes more history reachable, the next safe scalability slice is usually a render boundary in the client rather than a second backend contract.
- Testing lesson: UI scalability changes still benefit from pure helper tests when the product behavior can be described as list slicing and explicit operator counts instead of DOM-only assertions.
- Workflow lesson: a fresh worktree often needs its own dependency bootstrap before package checks mean anything, so record that install step explicitly instead of pretending the environment was already warm.

## Next Step
- Select the next post-release milestone from the remaining queued follow-ons.

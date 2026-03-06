# PR57 - Workbench Replay Control

## Metadata
- Date: 2026-03-06
- PR: 57
- Branch: `feat/pr57-workbench-replay-control`
- Title: add the first Workbench replay control by replaying the latest observed runtime probe payload
- Commit(s): pending

## Problem
- Workbench could send a canned runtime probe after PR56, but it still lacked any control that replayed previously observed activity from timeline history.
- The roadmap needed the first replay capability, but building a broader replay engine or new daemon-side API would have widened scope before the team had evidence about which replay workflows matter.

## Approach (with file+line citations)
- Change 1:
  - Why: extend the Workbench runtime summary so it can derive the latest replayable namespaced runtime payload and explain whether replay is sourced from the last result payload or the last `runtime.onMessage` payload.
  - Where: `inspector/src/workbench.ts:1-455`
  - Where: `inspector/tests/workbench.test.ts:1-258`
- Change 2:
  - Why: add a Workbench replay control that resends the latest observed runtime probe payload through the existing runtime transport and stays disabled until timeline history contains a replayable payload.
  - Where: `inspector/src/tabs/workbench.ts:21-229`
- Change 3:
  - Why: document the decision boundary that replay starts as a constrained Workbench control driven by actual timeline observations rather than a generic replay subsystem.
  - Where: `docs/adr/021-workbench-runtime-replay-control.md:1-31`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:57-64`
  - Where: `docs/build-log/2026-03-06-pr-057-workbench-replay-control.md:1-54`

## Risk and Mitigation
- Risk: replay could silently become a second transport or a fake local status mechanism.
- Mitigation: this control only reuses the existing `sendRuntimeMessage(...)` path and derives replay availability from actual timeline entries.
- Risk: a replay feature could look broader than it really is and create product confusion.
- Mitigation: the UI and ADR make the scope explicit: replay is limited to the latest observed namespaced runtime probe payload in this milestone.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Expected:
  - Workbench exposes a replay button for the latest observed runtime probe payload.
  - Replay remains disabled until timeline history contains a replayable namespaced runtime payload.
  - Replay uses the existing runtime transport without any daemon or protocol changes.

## Teach-back (engineering lessons)
- Design lesson: the first replay feature in a new product surface should usually be derived from something the system already observed, not from speculative replay abstractions.
- Testing lesson: replay semantics are easier to trust when the source payload and replay eligibility are pure functions over timeline state.
- Team workflow lesson: naming explicit replay boundaries in an ADR prevents a small UI milestone from smuggling in backend expansion.

## Next Step
- Decide whether to graduate the disabled Replay tab into a focused history-driven surface using the Workbench-proven replay pattern without widening daemon scope.

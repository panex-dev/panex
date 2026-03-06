# PR56 - Workbench Runtime Probe

## Metadata
- Date: 2026-03-06
- PR: 56
- Branch: `feat/pr56-workbench-runtime-probe`
- Title: add the first namespaced Workbench runtime probe on top of `runtime.sendMessage`
- Commit(s): pending

## Problem
- Workbench had one live tool after PR55, but it was still limited to storage and had not yet proven that the surface could drive a non-storage action.
- The roadmap needed the first runtime-oriented Workbench action, but a generic runtime console would have introduced a second interaction model before operator needs were clear.

## Approach (with file+line citations)
- Change 1:
  - Why: add a small inspector connection helper plus runtime-probe derived-state helpers so Workbench can send a canned `runtime.sendMessage` probe and summarize the last real result/event from timeline state.
  - Where: `inspector/src/connection.ts:46-188`
  - Where: `inspector/src/connection.ts:446-462`
  - Where: `inspector/src/workbench.ts:1-400`
  - Where: `inspector/tests/workbench.test.ts:1-234`
- Change 2:
  - Why: render a single live runtime probe in Workbench and keep the remaining roadmap tools explicitly deferred.
  - Where: `inspector/src/main.tsx:46-56`
  - Where: `inspector/src/tabs/workbench.ts:1-197`
- Change 3:
  - Why: document the decision boundary that Workbench runtime actions should start as constrained probes over existing transport instead of becoming a generic message shell immediately.
  - Where: `docs/adr/020-workbench-runtime-probe-first-slice.md:1-31`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:57-63`
  - Where: `docs/build-log/2026-03-06-pr-056-workbench-runtime-probe.md:1-56`

## Risk and Mitigation
- Risk: runtime tooling could turn into an unconstrained message console too early and fracture the Workbench interaction model.
- Mitigation: this slice adds exactly one namespaced probe with a canned payload and defers generic payload editing.
- Risk: the UI could claim runtime status that is not actually reflected in daemon behavior.
- Mitigation: the Workbench model derives last-result and last-event summaries from timeline entries rather than inventing optimistic local state.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Expected:
  - Workbench exposes one live runtime probe built on `runtime.sendMessage`.
  - The probe payload remains namespaced under `panex.workbench.runtime-probe`.
  - The tab shows the last observed runtime result and `runtime.onMessage` payload from timeline data.

## Teach-back (engineering lessons)
- Design lesson: when a product surface is still finding its shape, one constrained action is usually a better milestone than a generic console.
- Testing lesson: timeline-derived summaries are a reliable way to keep interactive UI honest because they reflect actual transport behavior rather than local optimism.
- Team workflow lesson: documenting “one probe only” as an ADR decision prevents a seemingly small runtime slice from turning into a hidden platform expansion.

## Next Step
- Add the first Workbench replay control on top of existing timeline state without widening daemon scope prematurely.

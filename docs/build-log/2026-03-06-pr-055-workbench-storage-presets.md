# PR55 - Workbench Storage Presets

## Metadata
- Date: 2026-03-06
- PR: 55
- Branch: `feat/pr55-workbench-storage-presets`
- Title: add reversible namespaced storage presets as the first actionable Workbench tool
- Commit(s): pending

## Problem
- Workbench existed as a real tab after PR54, but it was still informational only and offered no direct operator action.
- The roadmap needed the first actionable Workbench tool, but broadening daemon or protocol scope again would have been premature while the Workbench interaction model is still taking shape.

## Approach (with file+line citations)
- Change 1:
  - Why: extend the Workbench-derived model with reversible storage preset state so the UI can accurately show whether each preset is missing, customized, or already applied.
  - Where: `inspector/src/workbench.ts:1-251`
  - Where: `inspector/tests/workbench.test.ts:1-163`
- Change 2:
  - Why: wire Workbench to launch namespaced storage presets through the existing storage mutation callbacks and present them as the first live Workbench tool.
  - Where: `inspector/src/main.tsx:46-55`
  - Where: `inspector/src/tabs/workbench.ts:1-161`
  - Where: `inspector/src/styles.css:214-351`
  - Where: `inspector/src/styles.css:450-494`
- Change 3:
  - Why: document the scope boundary that Workbench becomes interactive by reusing existing transport rather than widening protocol/daemon behavior.
  - Where: `docs/adr/019-workbench-storage-presets-first-action.md:1-31`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:55-62`
  - Where: `docs/build-log/2026-03-06-pr-055-workbench-storage-presets.md:1-56`

## Risk and Mitigation
- Risk: a first Workbench action could mutate user storage too aggressively or make cleanup unclear.
- Mitigation: the presets are restricted to reversible `panex.workbench.*` keys and switch between apply, update, and remove based on live storage state.
- Risk: Workbench could start growing a parallel mutation framework that diverges from the existing inspector transport.
- Mitigation: this milestone only reuses the current `storage.set` and `storage.remove` callbacks and does not introduce new daemon or protocol behavior.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Expected:
  - Workbench exposes a live storage-preset card with reversible actions.
  - Preset actions only target `panex.workbench.*` keys.
  - No daemon or protocol changes are required for the new Workbench action to function.

## Teach-back (engineering lessons)
- Design lesson: the first interactive slice of a new product surface should usually reuse an existing transport before introducing new backend contracts.
- Testing lesson: derived-state helpers are a good place to encode action semantics like `missing`, `customized`, and `applied`, because they make UI behavior testable without fragile browser fixtures.
- Team workflow lesson: naming explicit non-goals in an ADR helps keep a roadmap milestone small when a feature is becoming interactive for the first time.

## Next Step
- Add the first Workbench runtime probe on top of the existing `runtime.sendMessage` transport without introducing a second interaction model too early.

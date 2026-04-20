# 2026-04-20 — Phase 1 L1: polymorphic Action

## What

Replaced the flat `Action {Type, Path, Description}` struct in `internal/plan` with an
`Action` interface. Each variant (currently `GenerateManifestAction`) owns its data,
its execute logic, and its rollback. `Apply` now executes actions sequentially and,
on the first failure, reverses every previously-completed action in reverse order
before transitioning the run to `failed`.

Three audit findings collapse to zero:

- **C1** (`applyGenerateManifest` overwrites for multi-target) — the per-target action
  carries its own `Path` and `Manifest` map, so two targets get two distinct files.
  Regression test: `TestApply_MultiTarget_WritesDistinctManifests`.
- **H5** (transition errors silently ignored) — `_ = run.Transition(...)` is replaced
  with explicit error handling that surfaces transition failures on the result, since
  a state-machine reject indicates a programmer bug, not a runtime condition.
- **H6** (apply has no rollback) — `rollbackExecuted` walks completed actions in
  reverse and calls `Rollback` on each reversible one, recording rollback steps in
  the run ledger. Regression test: `TestApply_RollbackOnFailure`.

## Why

The audit (`docs/phase1-audit.md` L1) called this out as the single biggest leverage
point for Phase 2 composability. Future actions (`install_dependency`, `run_script`,
`copy_file`) plug in by implementing the interface and registering a factory — no
new switch case in `Apply`, no new ad-hoc rollback logic. Each new action carries
its own undo, so the rollback path scales with the action set instead of being a
single per-type branch.

## Impact

- Wire format for `Plan.Actions` changed from flat `{type, path, description, ...}`
  to `{type, description, risk, reversible, spec: {...}}`. Plans are ephemeral
  (`current.plan.json` is regenerated on every `panex plan` invocation), so no
  migration is needed.
- `ApplyResult` now reports `RolledBack []string` alongside `Applied`/`Failed`.
- New action types must register a factory in `actionRegistry`.

## Quality

- `make fmt && make lint && make test && make build` — all green.
- New tests: `TestApply_MultiTarget_WritesDistinctManifests`,
  `TestApply_RollbackOnFailure`, `TestActionList_RoundTrip`,
  `TestActionList_UnknownType`.
- Existing plan tests pass unchanged.

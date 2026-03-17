# PR 105 — Deduplicate daemon URL utilities into shared/protocol

**Status:** merged
**Branch:** `feat/pr105-deduplicate-daemon-url`
**Base:** `main`

## What

Move `buildDaemonURL`, `nonEmpty`, `normalizeDaemonWebSocketURL`, and `loopbackHosts` from agent and inspector into `@panex/protocol`. Both packages now import these from the shared module instead of maintaining identical local copies.

## Why

These functions were duplicated between `agent/src/config.ts` and `inspector/src/connection.ts` with only trivial signature differences (`null` vs `undefined`). Keeping two copies creates drift risk — any fix applied to one must be manually replicated in the other. The shared protocol package is the natural home since both already depend on it.

## Changes

- `shared/protocol/src/index.ts` — export `buildDaemonURL`, `nonEmpty`, `normalizeDaemonWebSocketURL`; signature accepts `string | null | undefined` to cover both callers
- `agent/src/config.ts` — import from `@panex/protocol`, remove local duplicates, re-export `buildDaemonURL` for existing test imports
- `inspector/src/connection.ts` — import from `@panex/protocol`, remove local duplicates and `loopbackHosts` constant

## What was left in place

`shared/chrome-sim` has its own `nonEmpty` and `buildDaemonURL` with different return types (`string | undefined` vs `string`). These serve a different contract (optional values) and were not consolidated.

## Quality

- `shared/protocol`: check + 15 tests pass
- `agent`: check + 26 tests pass
- `inspector`: check + 62 tests pass
- `make fmt && make lint && make build` — clean

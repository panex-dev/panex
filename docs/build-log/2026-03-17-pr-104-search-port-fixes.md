# PR 104 — Fix type: operator inconsistency and reject privileged ports

**Status:** merged
**Branch:** `feat/pr104-inspector-config-fixes`
**Base:** `main`

## What

Make the inspector `type:` search operator use `includes` (matching `name:` and `src:`) and raise the minimum allowed server port from 1 to 1024.

## Why

The `type:` operator used `startsWith` while `name:` and `src:` used `includes`, making search behavior inconsistent and surprising. Port validation allowed 1-1023 which require root on Linux — a developer tool should default to unprivileged ports only.

## Changes

- `inspector/src/timeline.ts` — change `type:` clause from `startsWith` to `includes`
- `internal/config/config.go` — change `minPort` from `1` to `1024`

## Quality

- `make fmt && make lint && make test && make build` — pass
- `cd inspector && pnpm run check && pnpm run test` — pass

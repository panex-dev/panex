# PR 106 — Unify ContextLog timestamp to milliseconds

**Status:** merged
**Branch:** `feat/pr106-timestamp-ms-consistency`
**Base:** `main`

## What

Rename `ContextLog.TimestampS` (seconds) to `ContextLog.TimestampMS` (milliseconds) in both Go and TypeScript protocol types, aligning with every other time-based field in the protocol.

## Why

`ContextLog.TimestampS` was the only field using seconds — all other time fields use milliseconds (`recorded_at_ms`, `duration_ms`). This inconsistency risked silent bugs where consumers assume milliseconds but receive seconds, producing timestamps 1000x too small.

## Changes

- `internal/protocol/types.go` — rename `TimestampS int64 msgpack:"timestamp_s"` to `TimestampMS int64 msgpack:"timestamp_ms"`
- `internal/protocol/types_test.go` — update test fixture value from 10 to 10000
- `shared/protocol/src/index.ts` — rename `timestamp_s: number` to `timestamp_ms: number`
- `agent/tests/handshake.test.ts` — update test fixture value from 1 to 1000

## Impact

- Wire format changes from `timestamp_s` to `timestamp_ms` in msgpack encoding
- No production code currently populates this field (only set by extension agents)
- No inspector code reads this field (inspector uses `recorded_at_ms` from `EventSnapshot`)

## Quality

- `make fmt && make lint && make test && make build` — pass
- All TS packages: check + test — pass

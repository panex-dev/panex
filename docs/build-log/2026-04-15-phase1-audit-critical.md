# Phase 1 Audit — Critical Correctness Fixes

**Status:** in review
**Date:** 2026-04-15

## What

Fix 7 critical correctness issues found during the Phase 1 audit, addressing adapter extensibility, code duplication, placeholder implementations, unchecked errors, lint violations, and unreachable state machine states.

## Why

These issues block Phase 2 or silently produce incorrect behavior: hardcoded Chrome references prevent multi-target support, duplicate graph-loading logic risks divergence, placeholder stale lock detection never actually checks age, unchecked error returns cause silent data loss, and an unreachable ledger state creates dead code.

## Fixes

| # | Issue | File(s) | Fix |
|---|---|---|---|
| 1 | Chrome adapter hardcoded 5x in MCP | `target/adapter.go`, `mcp/mcp.go`, `cli/cli.go` | Added `target.Registry` with `DefaultRegistry()`, replaced all hardcoded maps |
| 2 | Duplicate `loadGraph` in MCP vs CLI | `cli/cli.go`, `mcp/mcp.go` | Exported `cli.LoadProjectGraph()`, MCP now delegates to it |
| 3 | Doctor stale lock check was placeholder | `doctor/doctor.go` | Implemented real `time.Since(modTime)` age check with 1-hour threshold |
| 4 | Doctor `os.Remove` unchecked in repair | `doctor/doctor.go` | Added error collection and reporting |
| 5 | Silent state/plan/session write failures | `cli/cli.go` | Checked all `_ =` error returns, propagated via Output.Errors/Warnings |
| 6 | Inspector `nilerr` lint violation | `inspector/inspector.go` | Split `err != nil || d.IsDir()` into separate checks, return `err` on error |
| 7 | Ledger `StatusExpired` unreachable | `ledger/ledger.go` | Added `StatusExpired` as valid transition from running/paused/awaiting-policy |

## Impact

- 10 files changed across 7 packages
- All 24 packages pass `go test -race`
- `make fmt` — clean
- `make build` — binary builds

## Quality

- `go test -race -count=1 ./internal/... ./cmd/panex/...` — 24/24 pass
- New adapter registry tests (`TestDefaultRegistry_ContainsChrome`, `TestRegistry_RegisterAndGet`)
- Doctor stale lock tests updated to use backdated file timestamps

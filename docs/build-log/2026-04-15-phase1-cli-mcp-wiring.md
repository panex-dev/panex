# Phase 1 CLI + MCP Wiring

**Status:** merged (direct to main)
**Commit:** pending
**Date:** 2026-04-15

## What

Wire all Phase 1 CLI commands to the `panex` binary and complete the 3 missing MCP tool handlers, closing the agent loop end-to-end.

## Why

The Phase 1 packages implemented all command logic in `internal/cli` and MCP tool definitions in `internal/mcp`, but:
- The binary (`cmd/panex/main.go`) only exposed `version`, `init`, `dev`, `doctor`, `paths` — the new Phase 1 commands (`inspect`, `plan`, `apply`, `test`, `verify`, `package`, `report`, `resume`, `mcp`) were not callable from the CLI.
- Three MCP tools (`repair_failure`, `resume_run`, `start_dev_session`) were declared in `tools/list` but had no handler implementations.

## New CLI Commands

| Command | Internal function | What it does |
|---|---|---|
| `panex inspect` | `cli.CmdInspect` | Scan project and report findings |
| `panex plan` | `cli.CmdPlan` | Compute proposed changes |
| `panex apply [--force]` | `cli.CmdApply` | Execute plan with drift detection |
| `panex test` | `cli.CmdTest` | Run verify + doctor |
| `panex verify` | `cli.CmdVerify` | Verification checks |
| `panex package [--version]` | `cli.CmdPackage` | Build distributable artifacts |
| `panex report [--run-id]` | `cli.CmdReport` | Read run report |
| `panex resume [--run-id]` | `cli.CmdResume` | Resume paused/failed run |
| `panex mcp` | `mcp.Server.Run` | Start MCP stdio server |

## Completed MCP Handlers

| Tool | Implementation |
|---|---|
| `repair_failure` | Runs `doctor.Run` with `Fix: true` |
| `resume_run` | Reads run from ledger, transitions to running then succeeded |
| `start_dev_session` | Provisions session via `session.New`, writes metadata |

## Impact

- 2 new files, 2 modified files
- All 23 internal packages + cmd/panex pass `go test -race`
- `panex help` lists all 15 commands
- Full agent loop now executable: `panex inspect → init → plan → apply → dev → test → verify → package → report`
- MCP parity: all 12 declared tools now have handlers

## Quality

- `go test -race -count=1 ./internal/... ./cmd/panex/...` — 24/24 pass
- `make fmt` — clean
- `make build` — binary builds successfully

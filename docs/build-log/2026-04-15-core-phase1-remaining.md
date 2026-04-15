# Phase 1 Remaining — Config loader, manifest, plan/apply, lock, session, MCP, CLI

**Status:** merged (direct to main)
**Commit:** `0974dd6`
**Date:** 2026-04-15

## What

Complete the Phase 1 package set by adding the remaining 6 packages and extending the CLI with all Phase 1 commands. This closes the dependency chain: an agent can now inspect → init → plan → apply → verify → test → package → report through both CLI and MCP surfaces.

## Why

The foundation packages (commit `94fe4b2`) provided the core types and engines but lacked the mutation model (plan/apply), concurrency control, manifest compilation, dev session management, and the MCP surface required by the spec. This commit completes the agent loop.

## New Packages

| Package | Spec | What it does |
|---|---|---|
| `internal/configloader` | pt11 | Loads `panex.config.json`, computes SHA-256 config hash. Phase 1: JSON only, TS eval deferred. |
| `internal/manifest` | pt18 | Compiles target-specific `manifest.json` from resolved capabilities. Permission authority enforced — rejects expansion outside capability compiler. |
| `internal/plan` | pt21 | Plan/apply model — plan computes proposed changes with graph snapshot hash, apply executes with drift rejection, lock acquisition, step recording. |
| `internal/lock` | pt36 | File-based mutual exclusion — project/session/publish lock types, PID liveness checks, stale lock recovery. |
| `internal/session` | pt23-24 | Dev session controller — browser launch with isolated profile, handshake validation (token, session, protocol version), lifecycle state machine. |
| `internal/mcp` | pt35 | MCP stdio server — JSON-RPC 2.0, 12 tools (inspect, init, plan, apply, verify, test, doctor, repair, package, report, resume, dev), 4 resources. |

## Extended Packages

| Package | What changed |
|---|---|
| `internal/cli` | Added 6 commands: `CmdPlan`, `CmdApply`, `CmdDev`, `CmdTest`, `CmdReport`, `CmdResume`. Added `loadProjectGraph` helper that falls back to inspector + config when graph file is absent. |

## Key Design Decisions

- **Drift detection**: Plan records graph hash at plan time. Apply recomputes and rejects if hash changed (unless `--force`).
- **Permission authority**: Manifest compiler validates every permission came from capability compiler output. Unknown permissions trigger `permission_expansion_outside_capability_compiler` error.
- **MCP parity**: Every MCP tool wraps the same internal logic as the corresponding CLI command. Same run ledger, same graph loading, same verification.
- **Lock recovery**: Stale locks detected via `kill -0` (signal 0) PID liveness check. Doctor can recover automatically.
- **Handshake contract**: Session validates ephemeral token, session ID, and protocol version. Five rejection outcomes per spec section 24.5.

## Impact

- 28 files changed, 3983 insertions
- All 23 packages pass `go test -race`
- `make fmt && make lint && make build` — clean (lint clean on all new code)
- Also applies gofmt/goimports formatting to existing packages

## Quality

- `go test -race -count=1 ./internal/...` — 23/23 pass
- `make fmt` — clean
- `make lint` — clean on new packages (pre-existing errcheck warnings in older packages untouched)
- `make build` — Go binary + agent + inspector all build

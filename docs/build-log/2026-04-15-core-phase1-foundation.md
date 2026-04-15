# Phase 1 Foundation — Core packages

**Status:** merged (direct to main)
**Commit:** `94fe4b2`
**Date:** 2026-04-15

## What

Implement the first 11 Go packages forming the Panex Core engine foundation, following the Phase 1 dependency chain from the engineering spec.

## Why

The existing codebase implements a dev-time inspector/daemon for Chrome extensions. The new Panex Core is a complete rewrite of the operational layer — agent-native, non-interactive, JSON-first — designed so that AI agents can build, test, and package extensions without human intervention. This commit lays the foundation that all subsequent Phase 1 work builds on.

## Packages

| Package | Spec | What it does |
|---|---|---|
| `internal/fsmodel` | pt10 | `.panex/` directory contract — path accessors, atomic state read/write, idempotent init |
| `internal/panexerr` | pt29 | Structured error taxonomy — 15 categories, rich error type with retry/repair/recipe metadata |
| `internal/inspector` | pt14 | Project scanner — detects framework, bundler, language, entrypoints, targets from filesystem |
| `internal/graph` | pt13 | Project graph builder — merges inspector findings + config, SHA-256 drift detection hash |
| `internal/target` | pt17 | Target adapter interface + Chrome adapter — 28 capabilities, env detection, manifest compilation, zip packaging |
| `internal/capability` | pt15-16 | Capability compiler — resolves semantic capabilities to per-target permissions via adapter catalog |
| `internal/policy` | pt12 | Policy engine — TOML policy with conservative defaults, evaluates actions against constraints |
| `internal/ledger` | pt22 | Run ledger — state machine lifecycle (created→planned→running→succeeded/failed), step recording |
| `internal/verify` | pt31 | Verification engine — graph completeness, capability blocks, entry completeness, permission diff |
| `internal/doctor` | pt29-30 | Diagnostics + repair — 6 checks, 3 auto-repairs, structured report |
| `internal/cli` | pt34 | CLI surface (partial) — inspect, init, doctor, verify, package commands with JSON output envelope |

## Impact

- 25 new files, 5442 lines of Go
- All 17 packages (6 existing + 11 new) pass `go test -race`
- No modifications to existing packages
- Coexists cleanly with existing daemon/build/protocol/store packages

## Quality

- `go test -race -count=1 ./internal/...` — 17/17 pass
- All new packages have contract tests
- Error taxonomy provides structured errors for every failure path

# Phase 3 MCP Configure Project

**Status:** implemented
**Date:** 2026-05-20

## What

Add the MCP `configure_project` tool for structured edits to JSON-authored Panex configuration, with graph refresh after each successful update.

## Why

The Phase 1 gap inventory still listed programmatic project configuration as a deferred MCP surface. Agents could add a target through a specialized tool, but they could not update project identity, entries, target enablement, capabilities, runtime, packaging, compatibility, feature, or publish config fields through a general MCP tool.

## Approach

- Added `cli.ConfigureProject` as the shared implementation for structured JSON config patches.
- The tool bootstraps `panex.config.json` from the existing graph when no authored config exists, but rejects rewrites of `panex.config.ts` because TypeScript config remains human-authored source.
- Config patches update typed project, entry, target, runtime, and packaging fields, plus object-shaped capabilities, compatibility, features, and publish sections.
- Successful updates write `panex.config.json`, rebuild `.panex/project.graph.json`, and return config/graph paths plus target summaries.
- MCP tests cover tool registration, JSON config bootstrapping, graph refresh, boolean false patching, and TypeScript config rejection.

## Risk and Mitigation

- Risk: broad config mutation could hide accidental no-op calls. Mitigation: the helper rejects empty patches.
- Risk: agents could overwrite TypeScript-authored config with generated JSON. Mitigation: the helper rejects `panex.config.ts` rewrites and leaves the source untouched.
- Risk: stale graph state after config edits. Mitigation: every successful configure call rebuilds the graph using the existing config loader, inspector, and graph builder path.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./internal/mcp -run 'TestToolsList|TestToolConfigureProject' -count=1`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` failed in this linked worktree before linting package bodies because Go VCS stamping returned `error obtaining VCS status: exit status 128`; reran the same lint gate with VCS stamping disabled.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build` failed in this linked worktree before building package bodies because Go VCS stamping returned `error obtaining VCS status: exit status 128`; reran the same build gate with VCS stamping disabled.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`

## Teach-Back

General MCP mutation tools should reuse CLI-level helpers when they change project state. That keeps config edits, graph rebuilds, and JSON output semantics aligned across human and agent entry points.

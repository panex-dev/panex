# Phase 3 MCP Run History Query

**Status:** implemented
**Date:** 2026-05-20

## What

Add the MCP `query_run_history` tool so agents can list `.panex/runs/*/run.json` ledger records with pagination and optional `status` / `operation` filters.

## Why

The Phase 1 gap inventory still listed paginated run history as a deferred MCP surface. Without it, MCP clients could read one report at a time but could not discover prior run IDs, inspect failed histories, or page through ledger records without direct filesystem access.

## Approach

- Registered `query_run_history` in the MCP tool list with `limit`, `offset`, `status`, and `operation` arguments.
- Implemented a read-only history query that loads run ledgers through `fsmodel.Root` and `ledger.ReadFromDir`, sorts newest-first by `started_at`, and returns summary records capped at 100 items.
- Added MCP tests for tool registration, empty ledgers, filtering, pagination, ordering, and surfaced failure metadata.
- Removed `query_run_history` from the deferred MCP gap inventory and recorded this Phase 3 increment in `STATUS.md`.

## Risk and Mitigation

- Risk: malformed run directories could hide ledger corruption. Mitigation: the query returns a contextual error instead of silently skipping unreadable run records.
- Risk: large run histories could produce oversized responses. Mitigation: requests are paginated and capped at 100 returned runs.
- Risk: ordering could vary across filesystems. Mitigation: results are sorted by `started_at`, with `run_id` as a stable tie-breaker.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `make check`
- `GOCACHE=/tmp/go-build go test ./internal/mcp -run 'TestToolsList|TestToolQueryRunHistory' -count=1`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint` failed in this linked worktree before linting package bodies because Go VCS stamping returned `error obtaining VCS status: exit status 128`; reran the same lint gate with VCS stamping disabled.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build` failed in this linked worktree before building package bodies because Go VCS stamping returned `error obtaining VCS status: exit status 128`; reran the same build gate with VCS stamping disabled.
- `GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build make build`

## Teach-Back

MCP read surfaces should use the same filesystem and ledger helpers as mutation paths. That keeps tool behavior aligned with the on-disk contract while avoiding a second ad hoc interpretation of `.panex`.

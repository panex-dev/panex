# Phase 2 runtime — session identity collection

**Status:** PR pending
**Date:** 2026-05-08

## Problem

The Phase 1 dev session model already persisted the browser profile directory and browser PID, but it did not persist the extension ID associated with that runtime. That left `session.json` incomplete for Phase 2 runtime identity work and made multi-surface tooling rely on inference instead of a recorded session identity.

## Approach

- Added `Graph.RuntimeExtensionID()` so the runtime layer derives one stable extension identifier from graph project identity in a single place.
- Added `ExtensionID` to `session.Options`, `session.Session`, and `Session.Info()`, so both persisted `session.json` metadata and CLI/MCP dev responses now expose the extension ID alongside the existing profile and PID fields.
- Plumbed that runtime extension ID through both dev-session entry points:
  - `panex dev`
  - MCP `start_dev_session`
- Added regression tests for:
  - graph extension-ID derivation,
  - session metadata persistence,
  - CLI dev session output,
  - MCP dev session output.

## Risk and mitigation

- Risk: projects without a persisted graph identity still have no stable extension ID to report.
- Mitigation: derive from `project.id` first and `project.name` second, without inventing a new default that could silently mislabel a real multi-target project.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/graph ./internal/session ./internal/cli ./internal/mcp`

## Next Step

Use the now-complete dev session identity metadata when wiring the broader Phase 2 Dev Bridge daemon flow, so bridge/inspector state can bind to an explicit recorded runtime identity instead of reconstructing it ad hoc.

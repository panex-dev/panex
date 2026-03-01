# ADR-006: Esbuild Integration Strategy for Extension Bundling

## Status
Accepted

## Context
Panex needs a fast build step on source changes to keep the save-to-reload loop responsive.
This build step should run in-process in the daemon and expose a structured result for protocol events.

## Decision
- Use `github.com/evanw/esbuild/pkg/api` directly from Go, not a subprocess wrapper.
- On each watcher batch, run a full project build using discovered JS/TS entry points under the source root.
- Emit `build.complete` events to connected clients with build id, success flag, duration, and changed files.
- Keep diagnostics in daemon logs for now; event-store/inspector integration will capture them in later steps.

## Consequences
- Build startup overhead is minimal because bundling runs in-process.
- Full-build-per-change is simple and deterministic for MVP, but may be more expensive than incremental contexts.
- Entry discovery based on file extensions can include more outputs than strictly needed; acceptable for the first loop.

## Reversibility
Medium.
The `build.complete` protocol shape stays stable, so we can later switch to incremental contexts, smarter entry mapping, or a different bundler backend without changing downstream consumers.

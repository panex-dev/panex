# ADR-007: Dev Agent Scaffold and Runtime Transport

## Status
Accepted

## Context
Panex needs a runtime endpoint inside Chrome that can receive daemon commands and apply them without manual interaction.
For the MVP loop, that means receiving `command.reload` and invoking extension reload immediately after a successful build.

## Decision
- Build the Dev Agent as a Manifest V3 extension with a background service worker.
- Use MessagePack over WebSocket for wire compatibility with daemon protocol envelopes.
- Keep agent configuration in `chrome.storage.local` and append auth token as a WebSocket query parameter to match daemon handshake requirements.
- Start with a single actionable command (`command.reload`) and reject malformed transport payloads at the message boundary.
- Standardize JS/TS package management on `pnpm` for this repository.

## Consequences
- The extension can participate in daemon orchestration with minimal UI surface area.
- Service worker lifecycle means reconnect behavior must be resilient to teardown/restart cycles.
- A narrow command surface reduces accidental behavior while protocol-level telemetry is still early.
- `pnpm` lockfile and workspace hygiene stay deterministic across contributors.

## Reversibility
Medium.
The envelope contract and command naming stay stable, so we can add richer commands/UI and alternate auth schemes later without replacing the current scaffold.
